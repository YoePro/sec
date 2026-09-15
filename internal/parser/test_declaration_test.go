package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestParseTopLevelTestDeclarations verifies that the parser retains source
// tests as declarations distinct from ordinary functions, including their
// human-readable identity and ordinary Sec statement body.
//
// Rules:
//   - rules/foundations/grammar.md — TestDeclaration production
//   - rules/tooling/testing.md — §5 "Test declaration syntax"
//   - rules/tooling/testing.md — §42.1 "Parser"
func TestParseTopLevelTestDeclarations(t *testing.T) {
	path := "../../testdata/parser/test_declarations_valid_test.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("test declaration diagnostics = %+v", result.Diagnostics)
	}
	if len(result.Program.Statements) != 3 {
		t.Fatalf("statement count = %d, want 3", len(result.Program.Statements))
	}

	wants := []string{"empty input is rejected", "cleanup is preserved"}
	for index, want := range wants {
		declaration, ok := result.Program.Statements[index+1].(*ast.TestDeclaration)
		if !ok {
			t.Fatalf("statement %d = %T, want *ast.TestDeclaration", index+1, result.Program.Statements[index+1])
		}
		if declaration.Name == nil || declaration.Name.Value != want {
			t.Errorf("test %d name = %#v, want %q", index, declaration.Name, want)
		}
		if declaration.Body == nil || len(declaration.Body.Statements) != 1 {
			t.Errorf("test %d body = %#v, want one statement", index, declaration.Body)
		}
	}
}

// TestNestedTestDeclarationIsRejected verifies that nested declaration syntax
// is consumed without losing the following statement in the enclosing body.
//
// Rules:
//   - rules/tooling/testing.md — §5.3 "Top-level only"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestNestedTestDeclarationIsRejected(t *testing.T) {
	path := "../../testdata/parser/nested_test_declaration_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if !result.HasErrors {
		t.Fatal("nested test declaration parsed without an error")
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.ID == diagnostics.ParserMisplacedKeyword && diagnostic.Primary.Lexeme == "test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing nested-test diagnostic: %+v", result.Diagnostics)
	}
	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if function.Body == nil || len(function.Body.Statements) != 2 {
		t.Fatalf("outer body was not retained: %#v", function.Body)
	}
	if _, ok := function.Body.Statements[1].(*ast.DiscardStatement); !ok {
		t.Fatalf("following statement = %T, want *ast.DiscardStatement", function.Body.Statements[1])
	}
}

// TestMalformedTestHeadersRetainDeclarations verifies committed declaration
// intent and recovery across missing names, forbidden parameters, and forbidden
// source-visible return types.
//
// Rules:
//   - rules/tooling/testing.md — §§5.4–5.6 test declaration restrictions
//   - rules/tooling/testing.md — §42.1 "Parser"
//   - rules/compiler/parser_recovery.md — "Declaration-header recovery"
func TestMalformedTestHeadersRetainDeclarations(t *testing.T) {
	path := "../../testdata/parser/malformed_test_declarations_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable test diagnostics: %+v", result)
	}
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.ID == diagnostics.ParserInvalidTestDeclaration {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("invalid-test diagnostics = %d, want 3: %+v", count, result.Diagnostics)
	}
	if len(result.Program.Statements) != 5 {
		t.Fatalf("statement count = %d, want 5", len(result.Program.Statements))
	}
	for index := 1; index <= 3; index++ {
		declaration, ok := result.Program.Statements[index].(*ast.TestDeclaration)
		if !ok || !declaration.Invalid || declaration.Body == nil {
			t.Errorf("statement %d did not retain invalid test and body: %#v", index, result.Program.Statements[index])
		}
	}
	later, ok := result.Program.Statements[4].(*ast.FunctionDeclaration)
	if !ok || later.Name == nil || later.Name.Value != "Later" {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements[4])
	}
}

// TestTestSpellingRemainsContextual verifies that test is still an identifier
// in function-name and ordinary call positions.
//
// Rules:
//   - rules/foundations/grammar.md — TestDeclaration production
//   - rules/tooling/testing.md — §5.2 "Grammar"
func TestTestSpellingRemainsContextual(t *testing.T) {
	path := "../../testdata/parser/test_contextual_identifier_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("contextual test diagnostics = %+v", result.Diagnostics)
	}
	if len(result.Program.Statements) != 3 {
		t.Fatalf("statement count = %d, want 3", len(result.Program.Statements))
	}
	callee := result.Program.Statements[1].(*ast.FunctionDeclaration)
	caller := result.Program.Statements[2].(*ast.FunctionDeclaration)
	call := caller.Body.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	if callee.Name.Value != "test" || call.Function == nil || call.Function.Value != "test" {
		t.Fatalf("test spelling was not retained as an identifier: callee=%#v call=%#v", callee.Name, call)
	}
}
