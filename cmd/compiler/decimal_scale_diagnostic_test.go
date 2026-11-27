package main

import (
	"bytes"
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// CLI JSON carries the same A2012 facts in occurrence and message arguments.
// Rules: rules/tooling/diagnostics.md — §§8,31A; MD-049 — §15(1).
func TestDecimalScaleDiagnosticJSON(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/decimal_scale_diagnostics/reduced.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "scale.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	var output bytes.Buffer
	reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	warnings := a.Warnings()
	if len(warnings) != 1 {
		t.Fatal(warnings)
	}
	reporter.semaDiagnostic(warnings[0], "scale information\n")
	reporter.finish()
	report := decodeOccurrenceDocument(t, output.String())
	if len(report.Occurrences) != 1 {
		t.Fatal(report)
	}
	item := report.Occurrences[0]
	if item.ID == nil || *item.ID != diagnostics.InferredDecimalScale || item.Arguments["selected_scale"] != "-1" || item.Message.Arguments["capacity_reduced"] != "true" || len(item.Related) != 1 {
		t.Fatal(item)
	}
}
