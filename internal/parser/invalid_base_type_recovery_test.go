package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Malformed qualified and unit-bearing base type references retain invalid
// nodes, their recoverable source facts, and the following declaration.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Initial invalid expression and type retention"
//   - rules/foundations/grammar.md — "Type-reference grammar", "Unit annotation"
func TestInvalidQualifiedAndUnitTypeReferencesAreRetained(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/invalid_base_type_references_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable type errors: %+v", result)
	}
	if len(result.Program.Statements) != 5 {
		t.Fatalf("statement count = %d, want module, three invalid functions, and Next: %#v", len(result.Program.Statements), result.Program.Statements)
	}

	for index, wantName := range []string{"Qualified", "UnitOnly", "UnitAnnotated"} {
		function, ok := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != wantName || len(function.Parameters) != 1 {
			t.Fatalf("invalid declaration %d was not retained: %#v", index, result.Program.Statements[index+1])
		}
		typ := function.Parameters[0].Type
		if typ == nil || !typ.Invalid || typ.Recovery == nil || typ.Recovery.DiagnosticID == "" {
			t.Fatalf("%s type reference was not marked invalid: %#v", wantName, typ)
		}
		switch wantName {
		case "Qualified":
			if typ.Name != "pkg" {
				t.Fatalf("qualified-name prefix was lost: %#v", typ)
			}
		case "UnitOnly":
			if !typ.UnitOnly || typ.Unit != "m/" {
				t.Fatalf("unit-only source facts were lost: %#v", typ)
			}
		case "UnitAnnotated":
			if typ.Name != "Speed" || typ.Unit != "m/" {
				t.Fatalf("annotated type source facts were lost: %#v", typ)
			}
		}
	}
	if next, ok := result.Program.Statements[4].(*ast.FunctionDeclaration); !ok || next.Name.Value != "Next" {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements[4])
	}
}
