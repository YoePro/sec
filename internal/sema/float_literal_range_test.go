package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The source matrix checks the final binary width in all literal contexts,
// preserving valid wide explicit contexts, subnormals and signed underflow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–19;
// rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §3(7).
func TestResolvedFloatLiteralRanges(t *testing.T) {
	const path = "../../testdata/sema/md066_literals/float_ranges.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if errors := p.Errors(); len(errors) != 0 {
				t.Fatal(errors)
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			errors := a.Analyze(program)
			expected := map[int]string{}
			for line, text := range strings.Split(string(data), "\n") {
				if strings.Contains(text, "// binary32") {
					expected[line+1] = "binary32"
				}
				if strings.Contains(text, "// binary64") {
					expected[line+1] = "binary64"
					if width == 32 && strings.Contains(text, " binary32") {
						expected[line+1] = "binary32"
					}
				}
				if width == 32 && strings.Contains(text, "// platform32") {
					expected[line+1] = "binary32"
				}
			}
			for _, e := range errors {
				want, ok := expected[e.Line]
				if !ok || e.ID != diagnostics.FloatLiteralOutOfRange || !strings.Contains(e.Message, want) || e.Help == "" || e.File != path || e.Column < 1 || e.EndColumn <= e.Column {
					t.Errorf("unexpected diagnostic: %+v", e)
				}
				delete(expected, e.Line)
			}
			if len(expected) != 0 {
				t.Errorf("missing diagnostics on lines: %v", expected)
			}
			// Reusing an analyzer must not retain errors or stale shaped facts.
			if again := a.Analyze(program); len(again) != len(errors) {
				t.Fatalf("repeat analysis: %v versus %v", again, errors)
			}
		})
	}
}
