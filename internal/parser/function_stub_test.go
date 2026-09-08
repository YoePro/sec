package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func TestUnimplementedFunctionDiagnostic(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/function_stubs_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), "stubs.sec")).Parse()
	if !result.HasErrors || result.Fatal || len(result.Diagnostics) != 6 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for i, name := range []string{"fnName", "Read", "Create", "AtEnd", "VoidStub", "BoolStub"} {
		d := result.Diagnostics[i]
		wantTip := []string{`{ return "" }`, `{ return "" }`, "valid value of the declared return type", "{ return 0 }", "body like `{}`", "{ return false }"}[i]
		if d.ID != diagnostics.ParserUnimplementedFunction || d.Message != "Unimplemented function "+name || !strings.Contains(d.Help, wantTip) || d.Primary.Lexeme != name || d.Primary.File != "stubs.sec" {
			t.Fatalf("wrong stub diagnostic: %+v", d)
		}
	}
	if len(result.Program.Statements) != 8 {
		t.Fatalf("lost declarations: %#v", result.Program.Statements)
	}
	stub := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if stub.Body != nil || stub.ReturnType.Name != "string" {
		t.Fatalf("stub signature not retained: %+v", stub)
	}
	next := result.Program.Statements[2].(*ast.FunctionDeclaration)
	if next.Name.Value != "Following" || next.Body == nil {
		t.Fatal("following function lost")
	}
	impl := result.Program.Statements[4].(*ast.ImplStatement)
	if len(impl.Members) != 3 || impl.Members[2].(*ast.FunctionDeclaration).Body == nil {
		t.Fatal("following impl method lost")
	}
}

func TestBodylessRequirementsAreNotFunctionStubs(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/bodyless_requirements_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if result.HasErrors {
		t.Fatalf("legal declaration or commented body rejected: %+v", result.Diagnostics)
	}
}
