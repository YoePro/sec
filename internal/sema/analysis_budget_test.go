package sema

import (
	"testing"

	"sec/internal/layout"
)

func TestParseAnalysisDepth(t *testing.T) {
	for input, want := range map[string]AnalysisDepth{
		"interactive": AnalysisInteractive,
		" Standard ":  AnalysisStandard,
		"DEEP":        AnalysisDeep,
	} {
		got, err := ParseAnalysisDepth(input)
		if err != nil || got != want {
			t.Fatalf("ParseAnalysisDepth(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ParseAnalysisDepth("unbounded"); err == nil {
		t.Fatal("unsupported analysis depth was accepted")
	}
}

func TestAnalyzerAnalysisDepthDefaultsAndBudgets(t *testing.T) {
	if got := NewAnalyzer().AnalysisDepth(); got != AnalysisStandard {
		t.Fatalf("compiler analyzer depth = %q, want standard", got)
	}
	interactive := NewAnalyzerWithDepth(AnalysisInteractive)
	if got := interactive.AnalysisDepth(); got != AnalysisInteractive {
		t.Fatalf("interactive analyzer depth = %q", got)
	}
	if got := interactive.AnalysisBudget().MaxSummaryIterations; got <= 0 {
		t.Fatalf("interactive summary limit = %d, want a finite positive limit", got)
	}
	if got := NewAnalyzerWithDepth(AnalysisDeep).AnalysisBudget().MaxSummaryIterations; got != 0 {
		t.Fatalf("deep fixed-point override = %d, want lattice-derived finite bound", got)
	}
}

// Analysis-only tooling combines the target scalar plan with a depth budget
// without changing target-sized types.
// Rule: rules/compiler/compiler_pipeline.md — § 65(2) same plan/Sema facts.
func TestScalarPlanAnalyzerKeepsDepthAndTargetWidth(t *testing.T) {
	analyzer := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: 32}, AnalysisDeep)
	if analyzer.AnalysisDepth() != AnalysisDeep || analyzer.AnalysisBudget() != analysisBudget(AnalysisDeep) {
		t.Fatalf("depth = %q budget = %+v", analyzer.AnalysisDepth(), analyzer.AnalysisBudget())
	}
	if analyzer.targetUintWidthBits != 32 {
		t.Fatalf("target width = %d, want 32", analyzer.targetUintWidthBits)
	}
	if standard := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64}); standard.AnalysisDepth() != AnalysisStandard {
		t.Fatalf("default scalar-plan depth = %q", standard.AnalysisDepth())
	}
}
