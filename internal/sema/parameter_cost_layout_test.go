package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// analyzeSourceForTargetRaw analyzes source under a registered target's plan,
// so copy cost comes from that plan's canonical layout.
func analyzeSourceForTargetRaw(t *testing.T, input, os, arch string) (*Analyzer, []Error) {
	t.Helper()
	program := parser.New(lexer.New(input)).ParseProgram()
	definition, ok := target.Find(target.Target{OS: os, Arch: arch})
	if !ok {
		t.Fatalf("no target %s-%s", os, arch)
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzerWithScalarPlan(plan)
	return analyzer, analyzer.Analyze(program)
}

// testParameterCostModel is the linux-amd64 plan's cost model for tests that
// call the recommendation policy directly.
func testParameterCostModel(t *testing.T) *parameterCostModel {
	t.Helper()
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	return NewAnalyzerWithScalarPlan(plan).parameterCostModel()
}

// TestParameterCostUsesResolvedLayout: copy cost is the canonical layout size
// of the active plan, including alignment padding the old estimate ignored;
// it differs between targets while semantic demand does not, each
// recommendation carries the plan's layout fingerprint for cache
// invalidation, and without a plan (or for descriptor types whose layout P24
// has not defined) the size is the structural estimate, labeled as such.
// Rules: rules/analysis/parameter_usage_analysis.md — §§ 64, 87(2)-(3), 92.
func TestParameterCostUsesResolvedLayout(t *testing.T) {
	source := `
module main

type Padded struct {
    flag: bool,
    wide: int64,
}

type Big struct {
    values: int[40],
}

fn ReadPadded(value: Padded) int64 { return value.wide }
fn ReadBig(value: Big) int { return value.values[0] }
`
	recommendation := func(a *Analyzer, name string) ParameterRecommendation {
		for _, r := range a.ParameterUsageAnalysis().Recommendations() {
			if r.CallableName == name {
				return r
			}
		}
		t.Fatalf("no recommendation for %s", name)
		return ParameterRecommendation{}
	}
	amd64, errs := analyzeSourceForTargetRaw(t, source, "linux", "amd64")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	armv7, errs := analyzeSourceForTargetRaw(t, source, "linux", "armv7")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if padded := recommendation(amd64, "ReadPadded"); !padded.SizeKnown || padded.EstimatedSizeBytes != 16 {
		t.Fatalf("Padded must use the resolved 16-byte layout with padding: %+v", padded)
	}
	wide, narrow := recommendation(amd64, "ReadBig"), recommendation(armv7, "ReadBig")
	if wide.EstimatedSizeBytes != 320 || narrow.EstimatedSizeBytes != 160 || wide.Status != "recommended" || narrow.Status != "recommended" {
		t.Fatalf("int[40] is 320 bytes on amd64 and 160 on armv7: %+v / %+v", wide, narrow)
	}
	if wide.LayoutIdentity == "" || wide.LayoutIdentity == narrow.LayoutIdentity || wide.LayoutIdentity != amd64.ParameterUsageAnalysis().RecommendationLayoutIdentity() {
		t.Fatalf("recommendations must carry the plan layout fingerprint: %q / %q", wide.LayoutIdentity, narrow.LayoutIdentity)
	}
	demandWide := parameterUsageSummaryNamed(t, amd64.ParameterUsageAnalysis(), "ReadBig").Parameters[0].Demand
	demandNarrow := parameterUsageSummaryNamed(t, armv7.ParameterUsageAnalysis(), "ReadBig").Parameters[0].Demand
	if demandWide.Access != demandNarrow.Access || demandWide.Ownership != demandNarrow.Ownership || demandWide.Precision != demandNarrow.Precision {
		t.Fatalf("semantic demand must not depend on the target layout: %+v / %+v", demandWide, demandNarrow)
	}
	planless, errs := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if r := recommendation(planless, "ReadBig"); r.SizeSource != ParameterSizeFromEstimate || r.LayoutIdentity != "" {
		t.Fatalf("without a plan the size is the labeled structural estimate: %+v", r)
	}
	if wide.SizeSource != ParameterSizeFromLayout || !strings.Contains(wide.CopyAdvice(), "Copy size on the active target: 320 bytes") {
		t.Fatalf("a resolved layout is the size source on a plan: %+v", wide)
	}
}
