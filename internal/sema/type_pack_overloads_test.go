package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Pack overloads use ordinary argument-only ranking, including fixed generic
// preference. Expected results never choose an overload, and argument effects
// execute once and commit only after unambiguous selection.
// Rules: rules/declarations/functions.md §38A(4)(5),(6)(1–2),§37;
// rules/declarations/generics.md §29; rules/memory/copy_move.md §8.3.
func TestTypePackOverloadSelection(t *testing.T) {
	for _, name := range []string{"overloaded_calls.sec", "overloaded_once.sec", "shared_return_overload_invalid.sec", "overloaded_owning_ambiguous_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		parsed := parser.New(lexer.New(string(data))).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		a := NewAnalyzer()
		errors := a.Analyze(parsed.Program)
		if strings.Contains(name, "invalid") {
			if len(errors) != 1 || errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable || !strings.Contains(errors[0].Message, "ambiguous call") {
				t.Fatal(name, errors)
			}
			ast.Visit(parsed.Program, func(node any) bool {
				if call, ok := node.(*ast.CallExpression); ok {
					if _, selected := a.ResolvedCallTarget(call); selected {
						t.Fatal("ambiguous call retained target", name)
					}
				}
				return true
			})
			continue
		}
		if len(errors) != 0 {
			t.Fatal(name, errors)
		}
		packCalls := 0
		ast.Visit(parsed.Program, func(node any) bool {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return true
			}
			target, selected := a.ResolvedCallTarget(call)
			if !selected {
				t.Fatal("missing selected target", name, call.String())
			}
			if target.Function.Name == "Fixed" {
				if target.Function.GenericTypePack != "" {
					t.Fatal("pack displaced equivalent fixed generic signature")
				}
				return true
			}
			fact, pack := a.ResolvedTypePackCallOf(call)
			if !pack {
				return true
			}
			packCalls++
			if fact.FixedCount != 1 {
				t.Fatal(fact)
			}
			if typ, known := a.ResolvedTypeOf(call); !known || typ.Kind != UintType {
				t.Fatal("missing call type", typ, known)
			}
			if name == "overloaded_calls.sec" {
				want := IntType
				if packCalls != 1 {
					want = BoolType
				}
				if fact.Parameters[0].Type.Kind != want {
					t.Fatal("wrong fixed-prefix candidate", fact)
				}
			}
			return true
		})
		want := 3
		if name == "overloaded_once.sec" {
			want = 1
		}
		if packCalls != want {
			t.Fatal(name, packCalls)
		}
	}
}

// The existing single-pack expected-result context must survive the overload
// change. A uniquely argument-selected template completes its scalar parameters
// from expected result context without using that context for selection.
// Rules: rules/declarations/generics.md §§18,29; rules/declarations/functions.md §38A(4)(5).
func TestTypePackOverloadExpectedContext(t *testing.T) {
	for _, name := range []string{"overloaded_expected_context.sec", "overloaded_result_context.sec"} {
		data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		parsed := parser.New(lexer.New(string(data))).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		a := NewAnalyzer()
		errors := a.Analyze(parsed.Program)
		if len(errors) != 0 {
			t.Fatal(name, errors)
		}
	}
}
