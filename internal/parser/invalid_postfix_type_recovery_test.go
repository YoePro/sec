package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Malformed postfix bracket type families retain an explicitly invalid type
// node and preserve the following top-level declaration.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Initial invalid expression and type retention"
//   - rules/compiler/parser_recovery.md — "Generic argument recovery"
func TestInvalidPostfixTypeReferencesAreRetained(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/invalid_postfix_type_references_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable type errors: %+v", result)
	}
	if len(result.Program.Statements) != 7 {
		t.Fatalf("statement count = %d, want module, five invalid functions, and Next: %#v", len(result.Program.Statements), result.Program.Statements)
	}
	for index, wantName := range []string{"FixedArray", "Collection", "MapValue", "GenericArgument", "EventCapacity"} {
		function, ok := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != wantName || len(function.Parameters) != 1 {
			t.Fatalf("invalid declaration %d was not retained: %#v", index, result.Program.Statements[index+1])
		}
		typ := function.Parameters[0].Type
		if typ == nil || !typ.Invalid || typ.Recovery == nil || typ.Recovery.DiagnosticID == "" {
			t.Fatalf("%s type reference was not marked invalid: %#v", wantName, typ)
		}
		if wantName == "FixedArray" && (typ.ElementType == nil || typ.ArrayLengthExpression == nil) {
			t.Fatalf("completed fixed-array suffix was discarded: %#v", typ)
		}
	}
	if next, ok := result.Program.Statements[6].(*ast.FunctionDeclaration); !ok || next.Name.Value != "Next" {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements[6])
	}
}
