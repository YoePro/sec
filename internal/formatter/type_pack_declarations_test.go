package formatter

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatTypePackDeclarations uses parser-owned declaration roles without
// changing the pack kind, reference mode, ordinary T or runtime spread grammar.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §10(3);
// rules/tooling/formatter.md — §§6,15–16,29.
func TestFormatTypePackDeclarations(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	input, want := read("formatting"), read("formatting.expected")
	kinds := func(source string) []bool {
		parsed := parser.New(lexer.New(source)).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		var packs []bool
		ast.Visit(parsed.Program, func(node any) bool {
			if p, ok := node.(*ast.GenericParameter); ok {
				packs = append(packs, p.TypePack)
			}
			return true
		})
		return packs
	}
	for _, options := range []Options{{}, {Fix: true}} {
		got := Format(Source{Text: input}, options).Text
		if got != want {
			t.Fatalf("formatted:\n%s\nwant:\n%s", got, want)
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Fatal("not idempotent", again)
		}
		if !reflect.DeepEqual(kinds(input), kinds(got)) {
			t.Fatal("pack became an ordinary parameter")
		}
	}
}
