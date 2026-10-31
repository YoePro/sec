package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Concrete fixed parameters keep ordinary literal compatibility, while pack
// inference remains independent and selected contextual calls publish their type.
// Rules: rules/declarations/functions.md §38A(4)(4–5),(9);
// rules/types/types.md — "Context shaping".
func TestTypePackFixedArgumentContext(t *testing.T) {
	for _, name := range []string{"shared_fixed_context.sec", "shared_fixed_context_invalid.sec"} {
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
			if len(errors) != 3 {
				t.Fatalf("want three incompatible fixed arguments, got %v", errors)
			}
			ast.Visit(parsed.Program, func(node any) bool {
				if call, ok := node.(*ast.CallExpression); ok {
					if _, resolved := a.ResolvedTypePackCallOf(call); resolved {
						t.Fatal("invalid call retained pack facts")
					}
				}
				return true
			})
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		calls := 0
		ast.Visit(parsed.Program, func(node any) bool {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return true
			}
			calls++
			typ, known := a.ResolvedTypeOf(call)
			if !known || typ.Kind != UintType {
				t.Fatalf("missing call type: %+v %v", typ, known)
			}
			fact, known := a.ResolvedTypePackCallOf(call)
			if !known || fact.FixedCount != 1 || fact.Parameters[0].Type.Name != "uint" {
				t.Fatal(fact, known)
			}
			if calls == 1 && (fact.Types[0].Name != "int" || fact.Types[1].Name != "bool") {
				t.Fatal("fixed context leaked into pack", fact)
			}
			return true
		})
		if calls != 3 {
			t.Fatal(calls)
		}
	}
}
