package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Composite type references propagate retained child/expression failures to
// the outer type without discarding their shape or following declarations.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Initial invalid expression and type retention"
//   - rules/compiler/parser_recovery.md — "Generic argument recovery"
func TestInvalidCompositeTypeReferencesPropagateRecovery(t *testing.T) {
	input, err := os.ReadFile("../../testdata/parser/invalid_composite_type_references_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.New(string(input))).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("expected recoverable type errors: %+v", result)
	}
	wantNames := []string{"CallableParameter", "CallableReturn", "PrefixElement", "ArrayLength", "ShapedConstant"}
	if len(result.Program.Statements) != len(wantNames)+2 {
		t.Fatalf("statement count = %d, want module, invalid functions, and Next: %#v", len(result.Program.Statements), result.Program.Statements)
	}
	for index, wantName := range wantNames {
		function, ok := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != wantName || len(function.Parameters) != 1 {
			t.Fatalf("invalid declaration %d was not retained: %#v", index, result.Program.Statements[index+1])
		}
		typ := function.Parameters[0].Type
		if typ == nil || !typ.Invalid || typ.Recovery == nil || typ.Recovery.DiagnosticID == "" {
			t.Fatalf("%s outer type was not marked invalid: %#v", wantName, typ)
		}
		switch wantName {
		case "CallableParameter":
			if len(typ.FunctionParameterTypes) != 1 || typ.FunctionParameterTypes[0] == nil || !typ.FunctionParameterTypes[0].Invalid {
				t.Fatalf("callable parameter recovery was lost: %#v", typ)
			}
		case "CallableReturn":
			if typ.FunctionReturnType == nil || !typ.FunctionReturnType.Invalid {
				t.Fatalf("callable return recovery was lost: %#v", typ)
			}
		case "PrefixElement":
			if typ.ElementType == nil || !typ.ElementType.Invalid {
				t.Fatalf("prefix element recovery was lost: %#v", typ)
			}
		case "ArrayLength":
			if typ.ElementType == nil || typ.ArrayLengthExpression == nil {
				t.Fatalf("array length recovery was lost: %#v", typ)
			}
		case "ShapedConstant":
			if typ.Name != "vector" || len(typ.TypeArgs) != 1 {
				t.Fatalf("shaped type prefix was lost: %#v", typ)
			}
		}
	}
	if next, ok := result.Program.Statements[len(wantNames)+1].(*ast.FunctionDeclaration); !ok || next.Name.Value != "Next" {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements[len(wantNames)+1])
	}
}
