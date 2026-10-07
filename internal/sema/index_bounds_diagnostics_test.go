package sema

import (
	"os"
	"reflect"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestIndexBoundsDiagnostics covers mandatory ownership, constant/direct
// coalescing, uncertain-before-known loops, all budgets, immutable evidence
// and reuse on real Sec sources. Suggestions must never become automatic fixes.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing",
// "Proven invalidity is not a warning", "Analysis states", "Fix safety".
func TestIndexBoundsDiagnostics(t *testing.T) {
	file := "../../testdata/sema/index_bounds_diagnostics_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		var reference []Error
		for _, budget := range []int{10000, 0, 1} {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if err := a.SetPitfallBudget(budget, budget); err != nil {
				t.Fatal(err)
			}
			errs := a.Analyze(program)
			assertPitfallBoundsErrorCount(t, errs, 4)
			for _, e := range errs {
				if e.File != file || e.Line <= 0 || e.EndColumn <= e.Column || !strings.Contains(e.Help, "pitfall.bounds.") || !strings.Contains(e.Help, "Suggested edit:") {
					t.Fatal(e)
				}
			}
			if reference == nil {
				reference = errs
			} else if !reflect.DeepEqual(errs, reference) {
				t.Fatal("budget changed owning diagnostics", errs, reference)
			}
			if budget == 10000 {
				facts := a.PitfallAnalysis().Findings()
				if len(facts) != 4 {
					t.Fatal(facts)
				}
				for _, fact := range facts {
					if fact.DiagnosticID != diagnostics.IndexOutOfBounds {
						t.Fatal(fact)
					}
					for _, action := range fact.Actions {
						if action.Kind != PitfallSuggestedEdit {
							t.Fatal(action)
						}
					}
				}
				facts[0].EvidenceFor[0].Fact = "mutated"
				if a.PitfallAnalysis().Findings()[0].EvidenceFor[0].Fact == "mutated" {
					t.Fatal("aliased evidence")
				}
			} else if len(a.PitfallAnalysis().Findings()) != 0 {
				t.Fatal("optional search escaped budget")
			}
			validFile := "../../testdata/sema/index_bounds_diagnostics_valid.sec"
			valid, err := os.ReadFile(validFile)
			if err != nil {
				t.Fatal(err)
			}
			p = parser.New(lexer.NewWithFile(string(valid), validFile))
			safe := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if safeErrors := a.Analyze(safe); len(safeErrors) != 0 {
				t.Fatal("safe path or stale proof rejected", safeErrors)
			}
		}
	}
}

// TestIndexBoundsUnreachable preserves owning unreachable-code errors while
// excluding unreachable direct-Len operations from bounds coalescing.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Diagnostic ownership and coalescing".
func TestIndexBoundsUnreachable(t *testing.T) {
	file := "../../testdata/sema/index_bounds_unreachable_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errs := NewAnalyzer().Analyze(program)
	if len(errs) != 3 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		if e.ID == diagnostics.IndexOutOfBounds || !strings.Contains(e.Message, "unreachable statement") {
			t.Fatal(e)
		}
	}
}
