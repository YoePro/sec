package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Mutable borrowed packs reuse ordinary per-argument exclusive loan checks;
// no mutable reference is copied or silently upgraded from shared authority.
// Rules: rules/declarations/functions.md §38A(3)(2–5),(4),(8);
// rules/memory/borrowing.md §15.2–15.4.
func TestMutableTypePackCalls(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"mutable_calls.sec", "mutable_overlap_invalid.sec", "mutable_reborrow_overlap_invalid.sec", "mutable_fixed_overlap_invalid.sec", "mutable_shared_invalid.sec", "mutable_immutable_invalid.sec", "mutable_move_invalid.sec"} {
			data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.New(lexer.New(string(data))).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			errors := a.Analyze(parsed.Program)
			if name != "mutable_calls.sec" {
				if len(errors) != 1 {
					t.Fatalf("want one authority error in %s, got %v", name, errors)
				}
				for _, err := range errors {
					if err.ID == diagnostics.GenericTypePackExpansionUnavailable {
						t.Fatalf("%s hit capability gate: %v", name, errors)
					}
				}
				if name == "mutable_move_invalid.sec" && (len(errors) != 1 || errors[0].ID != diagnostics.MoveIntoBorrowedParameter) {
					t.Fatal(errors)
				}
				ast.Visit(parsed.Program, func(node any) bool {
					if call, ok := node.(*ast.CallExpression); ok {
						if _, resolved := a.ResolvedTypePackCallOf(call); resolved {
							t.Fatal("invalid call retained pack facts", name)
						}
					}
					return true
				})
				continue
			}
			if len(errors) != 0 {
				t.Fatal(width, errors)
			}
			declaration := parsed.Program.Statements[1].(*ast.FunctionDeclaration)
			length := declaration.Body.Statements[0].(*ast.ReturnStatement).Value
			calls := 0
			ast.Visit(parsed.Program, func(node any) bool {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return true
				}
				calls++
				fact, known := a.ResolvedTypePackCallOf(call)
				if !known || len(fact.Types) != len(call.Arguments) {
					t.Fatal(fact, known)
				}
				constant, known := a.ResolvedTypePackConstantOf(call, length)
				if !known || constant.Integer == nil || constant.Integer.Uint64() != fact.Length {
					t.Fatal("lost specialized mutable-pack length", constant, known)
				}
				for i, parameter := range fact.Parameters {
					if parameter.Type.Kind != ReferenceType || !parameter.Type.ReferenceMutable || parameter.Type.Element.Kind == ReferenceType {
						t.Fatal("lost exclusive concrete receiving mode", parameter)
					}
					if plan, planned := a.ResolvedReferencePlanOf(call.Arguments[i]); planned {
						if !plan.Facts.MutableAuthority {
							t.Fatal("lost exclusive authority", i)
						}
					} else {
						if use, passed := a.ResolvedReferenceUseOf(call.Arguments[i]); !passed || !use.Facts.MutableAuthority {
							t.Fatal("missing ordinary borrow/reborrow plan", i)
						}
					}
				}
				return true
			})
			if calls != 4 {
				t.Fatal(calls)
			}
		}
	}
}
