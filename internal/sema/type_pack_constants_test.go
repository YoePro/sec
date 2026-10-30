package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Specialization constant evaluation is isolated by parameter identity and call,
// preserving short-circuiting and the active uint domain at every intermediate.
// Rules: rules/declarations/functions.md §38A(5)(2),(9);
// rules/compiler/compile_time_evaluation.md §§2(3),4(4),17,47.
func TestTypePackSpecializedLengthConstants(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/shared_length_constants.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		parsed := parser.New(lexer.New(string(data))).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		bodies := map[string]ast.Expression{}
		for _, statement := range parsed.Program.Statements {
			if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value != "Use" {
				if ret, ok := fn.Body.Statements[0].(*ast.ReturnStatement); ok {
					bodies[fn.Name.Value] = ret.Value
				}
			}
		}
		seen := 0
		ast.Visit(parsed.Program, func(node any) bool {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return true
			}
			fact, pack := a.ResolvedTypePackCallOf(call)
			if !pack {
				return true
			}
			seen++
			target, _ := a.ResolvedCallTarget(call)
			name := target.Function.Name
			value, known := a.ResolvedTypePackConstantOf(call, bodies[name])
			wantKnown := name != "RuntimeBody" && !(name == "Previous" && fact.Length == 0) && !(name == "Wide" && width == 32)
			if known != wantKnown {
				t.Fatalf("%d-bit %s/%d known=%v value=%+v", width, name, fact.Length, known, value)
			}
			if known {
				switch name {
				case "Extra":
					if value.Integer == nil || value.Integer.Uint64() != fact.Length+1 || value.Kind != UintType {
						t.Fatal(name, value)
					}
					value.Integer.SetUint64(99)
					again, _ := a.ResolvedTypePackConstantOf(call, bodies[name])
					if again.Integer.Uint64() != fact.Length+1 {
						t.Fatal("mutable specialization result")
					}
				case "Empty", "Guarded":
					want := name == "Guarded" || fact.Length == 0
					if value.Kind != BoolType || value.Bool != want {
						t.Fatal(name, value)
					}
				case "Previous":
					if value.Integer == nil || value.Integer.Sign() != 0 {
						t.Fatal(name, value)
					}
				case "Wide":
					if value.Integer == nil || value.Integer.String() != "4294967296" {
						t.Fatal(name, value)
					}
				}
			}
			other := "Extra"
			if name == other {
				other = "Empty"
			}
			if _, wrongBinding := a.ResolvedTypePackConstantOf(call, bodies[other]); wrongBinding {
				t.Fatal("same-spelled pack of another declaration was substituted")
			}
			return true
		})
		if seen != 11 {
			t.Fatal("missing selected pack calls", seen)
		}
	}
}
