package cst

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTypePackDeclarationRole keeps only the declaration's spread attached to
// its type-sequence name; ordinary runtime spread retains its own syntax role.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §10(3).
func TestTypePackDeclarationRole(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/roles.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	parsed := parser.New(lexer.New(source)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	document := Build(source, "pack.sec")
	document.ApplyProgramRoles(parsed.Program)
	if document.Text() != source {
		t.Fatal("lossless source changed")
	}
	count := 0
	for _, element := range document.Elements {
		if element.Token.Type == lexer.SPREAD && element.HasRole(PrefixOperator) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("declaration roles=%d, want one", count)
	}
}
