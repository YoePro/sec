package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTypeDiagnosticsSemanticMatrix verifies source-level type explanations
// independently of scalar width and advisory analysis depth.
// Rules: rules/types/types.md — Diagnostics; nominal identity, assignability,
// explicit conversions, generic types and fixed-array shape.
func TestTypeDiagnosticsSemanticMatrix(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/type_diagnostics/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth).Analyze(program)
			expected := map[int][]string{10: {"Left", "Right"}, 11: {"Small"}, 12: {"Box[int]", "Box[string]"}, 13: {"int[2]", "int[3]"}, 14: {"decimal<DiagnosticLength>", "decimal<DiagnosticTime>"}, 15: {"bool", "int"}}
			seen := map[int]bool{}
			for _, e := range errors {
				wants, ok := expected[e.Line]
				if !ok {
					t.Fatal(e)
				}
				seen[e.Line] = true
				for _, want := range wants {
					if !strings.Contains(e.Message, want) {
						t.Errorf("%+v missing %s", e, want)
					}
				}
				for _, backend := range []string{"LLVM", "MLIR", "i64", "layout"} {
					if strings.Contains(e.Message, backend) {
						t.Fatal(e)
					}
				}
				if e.Line == 11 && (e.ID != diagnostics.ValueViolatesContract || !strings.Contains(e.Message, "1..3")) {
					t.Fatal(e)
				}
				if e.Column == 0 {
					t.Fatal(e)
				}
			}
			if len(seen) != len(expected) {
				t.Fatal(errors)
			}

		}
	}
}
