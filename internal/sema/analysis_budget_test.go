package sema

import "testing"

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
