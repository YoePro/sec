package sema

import (
	"os"
	"strings"
	"testing"

	"math/big"
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

// TestNominalIdentityCacheKeys checks that ordinary comparison and callable/cache
// identity both distinguish declared names/modules, including structural carriers.
// Rules: rules/types/types.md — Type identity, Arrays, Function types.
func TestNominalIdentityCacheKeys(t *testing.T) {
	integer := builtinType("int")
	array := NewFixedArrayType(integer, big.NewInt(2))
	callback := Type{Kind: FunctionType, Name: "fn(int) int", FunctionParameterTypes: []Type{integer}, FunctionReturnType: &integer}
	for _, base := range []Type{integer, builtinType("bool"), builtinType("string"), array, callback} {
		left := base
		left.Name = "Identity"
		left.Named = true
		left.Module = "left"
		right := left
		right.Module = "right"
		sibling := left
		sibling.Name = "Sibling"
		for _, other := range []Type{right, sibling, base} {
			if sameConcreteType(left, other) || canonicalTypeIdentity(left) == canonicalTypeIdentity(other) {
				t.Fatalf("nominal collision: %s / %s", canonicalTypeIdentity(left), canonicalTypeIdentity(other))
			}
		}
		if !sameConcreteType(left, left) || canonicalTypeIdentity(left) != canonicalTypeIdentity(left) {
			t.Fatal("identity is not reflexive")
		}
	}
	placeholder := Type{Name: "Forward", Module: "main", Kind: InvalidType}
	complete := Type{Name: "Forward", Module: "main", Kind: StructType, Named: true}
	if !sameConcreteType(placeholder, complete) {
		t.Fatal("prepass declaration identity was lost")
	}
	complete.Module = "other"
	if sameConcreteType(placeholder, complete) {
		t.Fatal("prepass placeholder crossed a module boundary")
	}
}
