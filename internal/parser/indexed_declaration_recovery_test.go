package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMapAssignmentSyntaxKeepsDeclarationSeparate checks that malformed indexed
// declarations remain invalid nodes, preserve subsequent statements/declarations
// and never become assignments, including bodyless and locally handled try.
// Rules: rules/collections/collections.md — §14.5;
// rules/foundations/grammar.md — "Assignment statements", "Try assignment";
// rules/compiler/parser_recovery.md — "Assignment recovery".
func TestMapAssignmentSyntaxKeepsDeclarationSeparate(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/indexed_declaration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(data))).Parse()
	if len(result.Diagnostics) != 4 {
		t.Fatal(result.Diagnostics)
	}
	for _, d := range result.Diagnostics {
		if d.ID != diagnostics.ParserInvalidAssignmentExpr || d.Help == "" || (d.Primary.Type != lexer.DECLARE && d.Primary.Type != lexer.MOVE_DECLARE) {
			t.Fatal(d)
		}
	}
	for _, name := range []string{"Plain", "Fallible", "Move", "Handled"} {
		fn := functionNamed(result.Program, name)
		if fn == nil || len(fn.Body.Statements) != 2 {
			t.Fatal(name, fn)
		}
		invalid, ok := fn.Body.Statements[0].(*ast.InvalidStatement)
		if !ok || invalid.Recovery == nil || !invalid.Recovery.Reported || invalid.Recovery.End.ByteEnd <= invalid.Recovery.Start.ByteStart {
			t.Fatal(name, fn.Body.Statements)
		}
		if binding, ok := fn.Body.Statements[1].(*ast.LetStatement); !ok || binding.Name.Value != "followingValue" {
			t.Fatal(name, fn.Body.Statements)
		}
	}
	if functionNamed(result.Program, "Following") == nil {
		t.Fatal("lost following declaration")
	}
	data, err = os.ReadFile("../../testdata/parser/map_assignment_forms_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result = New(lexer.New(string(data))).Parse()
	if result.HasErrors {
		t.Fatal(result.Diagnostics)
	}
	fn := functionNamed(result.Program, "Forms")
	if len(fn.Body.Statements) != 4 {
		t.Fatal(fn)
	}
	if _, ok := fn.Body.Statements[0].(*ast.AssignmentStatement); !ok {
		t.Fatal(fn.Body.Statements)
	}
	for _, s := range fn.Body.Statements[1:3] {
		if _, ok := s.(*ast.TryAssignmentStatement); !ok {
			t.Fatal(s)
		}
	}
	if _, ok := fn.Body.Statements[3].(*ast.LetStatement); !ok {
		t.Fatal(fn.Body.Statements)
	}
}
