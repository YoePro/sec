package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestNonBindingMatchPayloadDiagnosticsLSP retains S1006, mandatory severity,
// source ranges and safe guidance from the shared Sema check.
// Rules: rules/control-flow/flowcontrol_match.md §8(5);
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestNonBindingMatchPayloadDiagnosticsLSP(t *testing.T) {
	file := "../../testdata/sema/match_payload_discards_invalid.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.NewWithFile(string(source), file)).ParseProgram()
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 17 {
		t.Fatalf("want 17 discard errors: %+v", errors)
	}
	for _, err := range errors {
		diagnostic := semaDiagnostic(err, 3, string(source))
		if diagnostic.Code != diagnostics.NonDiscardableValue || diagnostic.Severity != 1 || !strings.Contains(diagnostic.Message, "Bind the payload") || diagnostic.Range.Start.Line != err.Line-1 || diagnostic.Range.End.Character <= diagnostic.Range.Start.Character {
			t.Errorf("incomplete discard diagnostic: %+v", diagnostic)
		}
	}
}
