package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Owned pack positions use ordinary copies and explicit moves, committing no
// caller transfers until all arguments have been successfully prepared.
// Rules: rules/declarations/functions.md §38A(3)(2–3),(6);
// rules/memory/copy_move.md §§8.1–8.3,28.1.
func TestOwnedTypePackCalls(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"owned_calls.sec", "owned_unmarked_invalid.sec", "owned_reuse_invalid.sec", "owned_failed_preparation_invalid.sec", "owned_duplicate_move_invalid.sec", "owned_partial_overlap_invalid.sec", "owned_borrow_overlap_invalid.sec", "ordinary_duplicate_move_invalid.sec", "owned_reference_boundary.sec"} {
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
			if name != "owned_calls.sec" {
				if len(errors) != 1 {
					t.Fatalf("%s: want one error, got %v", name, errors)
				}
				if name == "owned_reference_boundary.sec" {
					if errors[0].ID != diagnostics.GenericTypePackExpansionUnavailable {
						t.Fatal(errors)
					}
				} else if errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable {
					t.Fatal("unexpected capability gate", name, errors)
				}
				if name != "owned_reuse_invalid.sec" {
					ast.Visit(parsed.Program, func(node any) bool {
						if call, ok := node.(*ast.CallExpression); ok {
							if _, selected := a.ResolvedTypePackCallOf(call); selected {
								t.Fatal("failed preparation retained selected call", name)
							}
						}
						return true
					})
				}
				if strings.Contains(name, "preparation") && strings.Contains(errors[0].Message, "moved") {
					t.Fatal("failed preparation committed earlier move", errors)
				}
				continue
			}
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			declaration := parsed.Program.Statements[1].(*ast.FunctionDeclaration)
			length := declaration.Body.Statements[0].(*ast.ReturnStatement).Value
			calls := 0
			ast.Visit(parsed.Program, func(node any) bool {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return true
				}
				fact, selected := a.ResolvedTypePackCallOf(call)
				if !selected {
					return true
				}
				calls++
				if fact.Length != uint64(len(call.Arguments)) {
					t.Fatal(fact)
				}
				constant, known := a.ResolvedTypePackConstantOf(call, length)
				if !known || constant.Integer == nil || constant.Integer.Uint64() != fact.Length {
					t.Fatal("lost owned length specialization", constant, known)
				}
				for i, parameter := range fact.Parameters {
					if parameter.Type.Kind == ReferenceType || !sameConcreteType(parameter.Type, fact.Types[i]) {
						t.Fatal("owned mode became a borrow", parameter)
					}
					argument := call.Arguments[i]
					if _, explicit := explicitMoveArgument(argument); explicit {
						plan, known := a.ResolvedTransferPlanOf(explicitMoveSource(argument))
						if !known || plan.Action != TransferMove {
							t.Fatal("missing ordinary move plan", plan, known)
						}
					} else if identifier, named := argument.(*ast.Identifier); named {
						plan, known := a.ResolvedTransferPlanOf(identifier)
						if !known || plan.Action != TransferCopyTrivial {
							t.Fatal("missing ordinary copy plan", plan, known)
						}
					}
				}
				return true
			})
			if calls != 6 {
				t.Fatal("missing owned calls", calls)
			}
		}
	}
}
