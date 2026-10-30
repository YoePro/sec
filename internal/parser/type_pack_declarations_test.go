package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestTypePackDeclarations distinguishes a trailing type sequence from T and
// retains the exact spread/name tokens without changing ordinary call grammar.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §§2–3.
func TestTypePackDeclarations(t *testing.T) {
	source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/declarations.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := New(lexer.New(string(source))).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	count := 0
	ast.Visit(parsed.Program, func(node any) bool {
		if parameter, ok := node.(*ast.GenericParameter); ok {
			if parameter.Name.Value == "T" {
				if parameter.TypePack || parameter.PackToken.Type != "" {
					t.Fatal("ordinary T became a pack")
				}
			} else {
				count++
				if !parameter.TypePack || parameter.PackToken.Type != lexer.SPREAD || parameter.Token.Type != lexer.IDENT {
					t.Fatalf("lost declaration kind: %+v", parameter)
				}
			}
		}
		return true
	})
	if count != 5 {
		t.Fatal("pack count", count)
	}
	callSource, err := os.ReadFile("../../testdata/generics/type_pack_declarations/empty_call.sec")
	if err != nil {
		t.Fatal(err)
	}
	calls := New(lexer.New(string(callSource))).Parse()
	if calls.HasErrors {
		t.Fatal("empty invocation grammar changed", calls.Diagnostics)
	}
}

// TestTypePackDeclarationRejections keeps the enclosing and following
// declarations while rejecting multiple/nonfinal packs and non-callable owners.
// Rules: rules/declarations/generics.md — §16A(2); MD-117 — §2(5).
func TestTypePackDeclarationRejections(t *testing.T) {
	for name, want := range map[string]string{"nonfinal": "must be last", "multiple": "only one type pack", "type_owner": "only on functions and methods"} {
		source, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name + "_invalid.sec")
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.New(string(source)))
		program := p.ParseProgram()
		if !strings.Contains(strings.Join(p.Errors(), "\n"), want) {
			t.Fatal(name, p.Errors())
		}
		found := false
		for _, stmt := range program.Statements {
			if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "After" {
				found = true
			}
		}
		if !found {
			t.Fatal("lost declaration after", name)
		}
	}
}
