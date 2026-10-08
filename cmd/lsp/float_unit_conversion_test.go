package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestFloatingUnitConversionDiagnosticsLSP keeps the same acceptance, semantic
// explanations and source ranges as Sema for exact and unproven conversions.
// Rules: rules/types/units.md — No hidden precision loss; rules/tooling/lsp.md — Shared diagnostic model.
func TestFloatingUnitConversionDiagnosticsLSP(t *testing.T) {
	for _, name := range []string{"valid.sec", "rejections_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/float_unit_conversion/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		errors := sema.NewAnalyzer().Analyze(program)
		expected := 0
		if name != "valid.sec" {
			expected = 8
		}
		if len(errors) != expected {
			t.Fatal(errors)
		}
		for _, e := range errors {
			d := semaDiagnostic(e, 3, string(data))
			if d.Range.Start.Line != e.Line-1 || d.Severity != 1 || !strings.Contains(d.Message, e.Message) {
				t.Fatal(d, e)
			}
		}
	}
}
