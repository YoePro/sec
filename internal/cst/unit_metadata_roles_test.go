package cst

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Qualified unit impls own metadata even across files. Same-spelled ordinary
// impl members and invalid numeric dots are not granted metadata compact roles.
// Rules: rules/foundations/grammar.md — "Unit metadata inside impl";
// rules/tooling/formatter.md §§5,22(11–13),29(2).
func TestUnitMetadataRolesRespectContext(t *testing.T) {
	source := "impl <external> {\nKind: Custom . Energy\nScale:1 . 5\n}\nimpl Ordinary {\nKind: Custom . Energy\n}\n"
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	document := Build(source, "")
	document.ApplyProgramRoles(program)
	names, compact := 0, 0
	for _, element := range document.Elements {
		if element.HasRole(UnitMetadataName) {
			names++
		}
		if element.HasRole(UnitMetadataCompactToken) {
			compact++
			if element.Token.Line != 2 || element.Token.Type != lexer.DOT {
				t.Fatalf("unowned compact role: %+v", element)
			}
		}
	}
	if names != 2 || compact != 1 {
		t.Fatalf("names=%d compact=%d", names, compact)
	}
	if document.Text() != source {
		t.Fatal("role annotation changed source")
	}
}
