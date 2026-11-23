package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
)

const floatCapabilitySource = `module main

fn Half(value: float64) float64 {
    return value / 2.0
}

fn Narrow(value: float32) float32 {
    return value
}

fn main() int {
    discard Half(1.0)
    discard Narrow(1.0)
    return 0
}
`

func analyzeFloatCapabilities(t *testing.T, plan layout.ResolvedScalarPlan) []Error {
	t.Helper()
	p := parser.New(lexer.NewWithFile(floatCapabilitySource, "floats.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return NewAnalyzerWithScalarPlan(plan).Analyze(program)
}

// P20 § 83: an LLVM-lowered target preserves IEEE-754 floats; a known plan
// lacking a required semantic rejects float use once per binary width at
// its first use; an unknown contract is not a failure.
func TestFloatTargetCapabilities(t *testing.T) {
	definition, _ := platformtarget.Find(platformtarget.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Float.Known || len(plan.Float.Missing(32)) != 0 || len(plan.Float.Missing(64)) != 0 {
		t.Fatalf("linux-amd64 float contract = %+v", plan.Float)
	}
	if errs := analyzeFloatCapabilities(t, plan); len(errs) != 0 {
		t.Fatalf("IEEE-preserving plan: %v", errs)
	}
	weak := plan
	weak.Float.Subnormals = false
	errs := analyzeFloatCapabilities(t, weak)
	if len(errs) != 2 {
		t.Fatalf("errors = %v", errs)
	}
	for index, width := range []string{"binary32", "binary64"} {
		if errs[index].ID != diagnostics.FloatTargetCapability || !strings.Contains(errs[index].Message, width) || !strings.Contains(errs[index].Message, "subnormal preservation") {
			t.Errorf("error %d = %+v", index, errs[index])
		}
	}
	unknown := plan
	unknown.Float = layout.FloatCapabilities{}
	if errs := analyzeFloatCapabilities(t, unknown); len(errs) != 0 {
		t.Fatalf("unknown contract: %v", errs)
	}
	for _, candidate := range platformtarget.Definitions() {
		resolved, err := candidate.ScalarPlan()
		if err != nil {
			continue
		}
		if (candidate.LLVMTriple != "") != resolved.Float.Known {
			t.Errorf("%s-%s float contract known = %v with triple %q", candidate.OS, candidate.Arch, resolved.Float.Known, candidate.LLVMTriple)
		}
	}
}
