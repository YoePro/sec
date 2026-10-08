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

// TestTypeDiagnosticsSemanticMatrixLSP preserves semantic explanations, source
// positions and mandatory reserved-unit diagnostics in the protocol adapter.
// Rules: rules/types/types.md — Diagnostics; rules/types/units.md — Future unit-polymorphic generics;
// rules/tooling/lsp.md — Shared diagnostic model.
func TestTypeDiagnosticsSemanticMatrixLSP(t *testing.T) {
	for _, name := range []string{"type_diagnostics/invalid.sec", "unit_parameters_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		errors := sema.NewAnalyzer().Analyze(program)
		expected := 6
		if name == "unit_parameters_invalid.sec" {
			expected = 5
		}
		if len(errors) != expected {
			t.Fatalf("%s: %v", name, errors)
		}
		for _, e := range errors {
			d := semaDiagnostic(e, 3, source)
			if d.Severity != 1 || d.Range.Start.Line != e.Line-1 || d.Range.Start.Character != e.Column-1 || !strings.Contains(d.Message, e.Message) {
				t.Fatal(d, e)
			}
			if name == "unit_parameters_invalid.sec" && (d.Code != diagnostics.UnitPolymorphismReserved || !strings.Contains(d.Message, "concrete declared unit")) {
				t.Fatal(d, e)
			}
		}
	}
}
