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
		for _, name := range []string{"owned_calls.sec", "owned_unmarked_invalid.sec", "owned_reuse_invalid.sec", "owned_failed_preparation_invalid.sec", "owned_duplicate_move_invalid.sec", "owned_partial_overlap_invalid.sec", "owned_borrow_overlap_invalid.sec", "ordinary_duplicate_move_invalid.sec", "owned_reference_move_invalid.sec"} {
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
				if errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable {
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

// A reference element of an owned pack is a by-value generic argument holding a
// reference, as T = ref int is for one generic parameter: it borrows for the
// call with ordinary overlap checks, a holder is reborrowed rather than moved,
// and a by-value read needs an implicitly copyable holder.
// Rules: rules/declarations/functions.md §38A(2)(3),(3)(2–4),(7)(6); rules/memory/references.md §3.4(10).
func TestOwnedTypePackReferenceHolders(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/owned_reference_holders.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(data)); len(errors) != 0 {
		t.Fatal(errors)
	}
	const count = "module main\nfn Count[...T](values: ...T) uint { return values.Len }\n"
	for _, item := range []struct{ source, want string }{
		{"fn M() uint {\n let mut a := 1\n let g := ref mut a\n return Count(<-g)\n}", "Cannot move 'g' into borrowed parameter"},
		{"fn M() uint {\n let mut a := 1\n return Count(ref mut a, ref mut a)\n}", "overlaps mutable borrow"},
		{"fn M() uint {\n let mut a := 1\n return Count(ref mut a, ref a)\n}", "overlaps mutable borrow"},
		{"fn L[...T](values: ...T) void { for v in values { } }\nfn M() void {\n let mut a := 1\n L(ref mut a)\n}", "position 0 is ref mut int"},
		{"fn One(a: int) uint { return 0 }\nfn F[...T](values: ...ref T) uint { return One(values...) }\nfn M() uint {\n let a := 1\n return F(a)\n}", "never dereferenced implicitly"},
		{"fn One(a: int) uint { return 0 }\nfn F[...T](values: ...T) uint { return One(values...) }\nfn M() uint {\n let a := 1\n return F(ref a)\n}", "never dereferenced implicitly"},
	} {
		messages := ""
		for _, err := range analyzeSourceRaw(t, count+item.source+"\n") {
			messages += err.Message + "\n"
		}
		if !strings.Contains(messages, item.want) {
			t.Errorf("%s: want %q, got %s", item.source, item.want, messages)
		}
	}
}
