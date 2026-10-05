package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Budgets affect optional coverage, never validity or the evidence needed for
// a finding. Whole-unit admission keeps suppressing evidence together.
// Rules: rules/compiler/compiler_analysis.md — §15(3–5);
// rules/analysis/pitfall_analysis.md — "Analysis states", "Interactive, Standard, and Deep analysis".
func TestPitfallBudgetCoverageAndSafety(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			analyze := func(path string, nodes, nesting int) (*Analyzer, []Error) {
				source, err := os.ReadFile("../../testdata/sema/" + path)
				if err != nil {
					t.Fatal(err)
				}
				lex := lexer.New(string(source))
				parsed := parser.New(lex)
				program := parsed.ParseProgram()
				if errors := parsed.Errors(); len(errors) != 0 {
					t.Fatal(errors)
				}
				analyzer := NewAnalyzerWithDepth(depth)
				if err := analyzer.SetPitfallBudget(nodes, nesting); err != nil {
					t.Fatal(err)
				}
				return analyzer, analyzer.Analyze(program)
			}
			full, errors := analyze("pitfall_budget_valid.sec", 10000, 100)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if got := len(full.PitfallAnalysis().Findings()); got != 63 {
				t.Fatalf("complete findings = %d", got)
			}
			limited, errors := analyze("pitfall_budget_valid.sec", 100, 100)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			analysis := limited.PitfallAnalysis()
			if got := len(analysis.Findings()); got != 1 {
				t.Fatalf("partial findings = %d, want complete first body only", got)
			}
			if coverage := analysis.Coverage(); coverage.SkippedUnits != 2 || coverage.VisitedNodes != 100 {
				t.Fatalf("coverage = %+v", coverage)
			}
			for _, evaluation := range analysis.Evaluations() {
				if evaluation.State == PitfallStateNoFinding {
					t.Fatalf("incomplete search claims NoFinding: %+v", evaluation)
				}
				if evaluation.Rule == PitfallBooleanLiteralComparison && (!evaluation.Incomplete || evaluation.FindingCount != 1 || evaluation.State != PitfallStateFinding) {
					t.Fatalf("partial finding lost coverage: %+v", evaluation)
				}
			}
			// Snapshot mutation and a fresh analysis must not retain old exhaustion.
			snapshot := limited.PitfallAnalysis()
			snapshot.evaluations[0].Incomplete = false
			snapshot.coverage.VisitedNodes = -1
			if !reflect.DeepEqual(analysis, limited.PitfallAnalysis()) {
				t.Fatal("snapshot mutation changed analyzer coverage")
			}
			source, err := os.ReadFile("../../testdata/sema/pitfall_budget_valid.sec")
			if err != nil {
				t.Fatal(err)
			}
			program := parser.New(lexer.New(string(source))).ParseProgram()
			if errors := limited.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			if !reflect.DeepEqual(analysis, limited.PitfallAnalysis()) {
				t.Fatal("same-budget analysis was not deterministic")
			}
			if err := limited.SetPitfallBudget(10000, 100); err != nil {
				t.Fatal(err)
			}
			if errors := limited.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			if limited.PitfallAnalysis().Coverage().SkippedUnits != 0 || len(limited.PitfallAnalysis().Findings()) != 63 {
				t.Fatal("old coverage persisted across Analyze")
			}
			for _, evaluation := range limited.PitfallAnalysis().Evaluations() {
				if evaluation.Incomplete {
					t.Fatal("old incomplete state persisted")
				}
			}

			completeNested, errors := analyze("pitfall_budget_nested_valid.sec", 10000, 100)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if results := completeNested.PitfallAnalysis().Results(); len(results) != 2 || results[0].State != PitfallStateSuppressed {
				t.Fatalf("full suppressing proof = %+v", results)
			}
			nested, errors := analyze("pitfall_budget_nested_valid.sec", 10000, 6)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if coverage := nested.PitfallAnalysis().Coverage(); coverage.SkippedUnits != 1 {
				t.Fatalf("nested coverage = %+v", coverage)
			}
			if findings := nested.PitfallAnalysis().Findings(); len(findings) != 1 || findings[0].Rule != PitfallBooleanLiteralComparison {
				t.Fatalf("partial nested proof leaked: %+v", findings)
			}
			zero, errors := analyze("pitfall_budget_valid.sec", 0, 0)
			if len(errors) != 0 || len(zero.PitfallAnalysis().Findings()) != 0 {
				t.Fatalf("zero-budget validity/findings: %v", errors)
			}
			invalid, invalidErrors := analyze("pitfall_budget_invalid.sec", 0, 0)
			_, fullErrors := analyze("pitfall_budget_invalid.sec", 10000, 100)
			if len(invalidErrors) == 0 || !reflect.DeepEqual(invalidErrors, fullErrors) || len(invalid.PitfallAnalysis().Findings()) != 0 {
				t.Fatalf("budget changed normative errors: %v / %v", invalidErrors, fullErrors)
			}
		})
	}
}

func TestPitfallBudgetDefaultsAndConfiguration(t *testing.T) {
	previousNodes, previousDepth := 0, 0
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer := NewAnalyzerWithDepth(depth)
		budget := analyzer.AnalysisBudget()
		if budget.MaxPitfallNodes <= previousNodes || budget.MaxPitfallDepth <= previousDepth {
			t.Fatalf("non-increasing budget: %+v", budget)
		}
		previousNodes, previousDepth = budget.MaxPitfallNodes, budget.MaxPitfallDepth
		if err := analyzer.SetPitfallBudget(-1, 1); err == nil || analyzer.AnalysisBudget() != budget {
			t.Fatal("negative budget changed configuration")
		}
		if err := analyzer.SetPitfallBudget(1, -1); err == nil || analyzer.AnalysisBudget() != budget {
			t.Fatal("negative depth changed configuration")
		}
	}
}
