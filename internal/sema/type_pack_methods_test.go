package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Concrete-owner pack methods retain ordinary receiver authority and reject
// absent/immutable/overlapping receivers before registering successful calls.
// Rules: rules/declarations/functions.md §§22–24,38A(3–4);
// rules/memory/borrowing.md §§7(5),15.2–15.4.
func TestTypePackMethodCalls(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"method_calls.sec", "method_reference_return.sec", "method_generic_owner_boundary.sec", "method_scalar_prefix_boundary.sec", "method_receiver_immutable_invalid.sec", "method_receiver_overlap_invalid.sec", "method_missing_receiver_invalid.sec", "method_receiver_escape_invalid.sec", "method_receiver_move_invalid.sec"} {
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
			if name != "method_calls.sec" && name != "method_reference_return.sec" {
				if len(errors) != 1 {
					t.Fatal(name, errors)
				}
				if strings.Contains(name, "boundary") {
					if errors[0].ID != diagnostics.GenericTypePackExpansionUnavailable {
						t.Fatal(errors)
					}
				} else if errors[0].ID == diagnostics.GenericTypePackExpansionUnavailable {
					t.Fatal(name, errors)
				}
				continue
			}
			if len(errors) != 0 {
				t.Fatal(name, errors)
			}
			calls := 0
			ast.Visit(parsed.Program, func(node any) bool {
				call, ok := node.(*ast.CallExpression)
				if !ok {
					return true
				}
				fact, known := a.ResolvedTypePackCallOf(call)
				if !known {
					return true
				}
				calls++
				target, _ := a.ResolvedCallTarget(call)
				if target.Kind != ResolvedStaticMethodCall {
					t.Fatal("lost method dispatch kind", target)
				}
				if target.Function.ImplTarget != "Counter" || fact.Length != uint64(len(call.Arguments)) {
					t.Fatal(target, fact)
				}
				return true
			})
			want := 7
			if name == "method_reference_return.sec" {
				want = 1
			}
			if calls != want {
				t.Fatal("missing method pack calls", calls)
			}
		}
	}
}
