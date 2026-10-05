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

// Deep reporting retains findings from complete units and explains missing
// coverage without inventing an error for optional budget exhaustion.
// Rule: rules/analysis/pitfall_analysis.md — "Analysis states", "sec analyse".
func TestAnalyseReportsIncompletePitfallCoverage(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_budget_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.New(string(source))).ParseProgram()
	analyzer := sema.NewAnalyzerWithDepth(sema.AnalysisDeep)
	if err := analyzer.SetPitfallBudget(100, 100); err != nil {
		t.Fatal(err)
	}
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	report := &analyseReport{counts: map[analyseReportClass]int{}}
	report.addPitfalls(analyzer.PitfallAnalysis(), func(lexer.Token) bool { return true })
	var output bytes.Buffer
	report.write(&output)
	for _, want := range []string{"incomplete coverage: 2 unit(s) skipped", "syntax search 100/100 nodes", "incomplete pitfall.boolean.redundant-literal-comparison (finding)", "(not-evaluated)"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("report missing %q: %s", want, output.String())
		}
	}
	if report.counts[analyseClassError] != 0 || report.counts[analyseClassAdvisory] != 1 {
		t.Fatalf("budget changed report classes: %+v", report.counts)
	}
}
