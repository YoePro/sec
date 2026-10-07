package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/compiler/compiler_analysis.md — §§15(5), 61(2–3);
// rules/analysis/parameter_usage_analysis.md — "Analysis budgets", "sec analyse".
func TestAnalyseParameterBudgetCoverage(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_budget_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.New(string(source))).ParseProgram()
	analyzer := sema.NewAnalyzerWithDepth(sema.AnalysisDeep)
	budget := analyzer.ParameterUsageBudget()
	budget.MaxRecommendationWork = 3
	if err := analyzer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	report := &analyseReport{counts: map[analyseReportClass]int{}}
	report.addParameterUsage(analyzer.ParameterUsageAnalysis(), func(lexer.Token) bool { return true })
	var output bytes.Buffer
	report.write(&output)
	text := output.String()
	for _, expected := range []string{"incomplete recommendation coverage: 2 candidate(s) skipped", "work 3/3", "First(values): candidate ref int[100], recommended", "Second(values int[100]): access read", "Third(values int[100]): access read", "0 errors, 0 unproven"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in report:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "Second(values): candidate") || strings.Contains(text, "Third(values): candidate") {
		t.Fatal("report invented skipped recommendation", text)
	}
}
