package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestSwitchStructPatternLSP preserves the parser's focused pattern error,
// help and type-head span rather than presenting downstream parser cascades.
// Rules: rules/control-flow/flowcontrol_switch.md §§28, 40;
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestSwitchStructPatternLSP(t *testing.T) {
	path := "../../testdata/parser/switch_patterns/qualified_invalid.sec"
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(input)
	result := parser.New(lexer.NewWithFile(text, path)).Parse()
	if len(result.Diagnostics) != 1 {
		t.Fatalf("parser errors = %+v", result.Diagnostics)
	}
	value := result.Diagnostics[0]
	got := structuredParserDiagnostic(value, text)
	if got.Code != diagnostics.ParserInvalidPattern || got.Severity != 1 || !strings.Contains(got.Message, "Use match") || got.Range != diagnosticTokenRange(value.Primary, text) || got.Range.End.Character-got.Range.Start.Character != len("Event") {
		t.Fatalf("incorrect LSP diagnostic: %+v", got)
	}
}
