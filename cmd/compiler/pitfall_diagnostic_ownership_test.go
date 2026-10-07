package main

import (
	"bytes"
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestPitfallDiagnosticOwnershipCLI checks registered owner transport and
// excludes supporting findings from diagnostic counts in the analysis report.
// Rules: rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing".
func TestPitfallDiagnosticOwnershipCLI(t *testing.T) {
	file := "../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec"
	for _, command := range []string{"sema", "analyse"} {
		_, output, code := runCLIForDiagnostics(t, command, file, "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || doc.Summary.Errors != 3 || len(doc.Occurrences) != 3 {
			t.Fatal(code, output)
		}
		for _, e := range doc.Occurrences {
			if e.Unregistered || len(e.Help) != 1 || !strings.Contains(e.Help[0].Text, "pitfall.") || len(e.Fixes) != 0 {
				t.Fatal(e)
			}
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	analyzer := sema.NewAnalyzerWithDepth(sema.AnalysisDeep)
	analyzer.Analyze(program)
	report := buildAnalyseReport(analyzer, map[string]bool{file: true}, hostCompilerTarget())
	var output bytes.Buffer
	report.write(&output)
	if strings.Count(output.String(), "not a second diagnostic") != 3 || report.counts[analyseClassError] != 0 {
		t.Fatal(output.String(), report.counts)
	}
}
