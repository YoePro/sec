package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMissingInfixOperandPreservesBlockBoundary verifies that an absent right
// operand does not consume the function body's closing brace or lose the next
// declaration.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Missing infix right operand"
//   - rules/compiler/parser_recovery.md — "Expression recovery"
func TestMissingInfixOperandPreservesBlockBoundary(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Broken(left: int) int {
	return left +
}
fn Next() int { return 1 }
`, "operand.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing infix operand must remain a parser error")
	}

	broken := functionNamed(result.Program, "Broken")
	if broken == nil || broken.Body == nil || len(broken.Body.Statements) != 1 {
		t.Fatalf("lost Broken function or return: %#v", broken)
	}
	returned, ok := broken.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Broken statement = %T, want return", broken.Body.Statements[0])
	}
	infix, ok := returned.Value.(*ast.InfixExpression)
	if !ok || infix.Left.String() != "left" || infix.Operator != "+" {
		t.Fatalf("return value = %#v, want retained left + expression", returned.Value)
	}
	invalid, ok := infix.Right.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.End.Type != lexer.RBRACE {
		t.Fatalf("right operand = %#v, want invalid expression ending at untouched brace", infix.Right)
	}
	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
	if !hasParserDiagnostic(result.Diagnostics, diagnostics.ParserInvalidExpression, lexer.PLUS) {
		t.Fatalf("diagnostics = %+v, want invalid-expression at plus", result.Diagnostics)
	}
}

// TestMissingOperandsPreserveCommaDelimitedSiblings verifies that both infix
// and prefix recovery stop before commas so later array elements survive.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Missing prefix operand"
//   - rules/compiler/parser_recovery.md — "Missing infix right operand"
//   - rules/compiler/parser_recovery.md — "Expression recovery"
func TestMissingOperandsPreserveCommaDelimitedSiblings(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Values(left: int) int[] {
	return [left +, 2, -, 4]
}
`, "operand.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing operands must remain parser errors")
	}

	function := functionNamed(result.Program, "Values")
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost Values function: %#v", function)
	}
	returned := function.Body.Statements[0].(*ast.ReturnStatement)
	array, ok := returned.Value.(*ast.ArrayLiteral)
	if !ok || len(array.Elements) != 4 {
		t.Fatalf("return value = %#v, want four retained array positions", returned.Value)
	}
	infix, ok := array.Elements[0].(*ast.InfixExpression)
	if !ok {
		t.Fatalf("first element = %T, want InfixExpression", array.Elements[0])
	}
	if invalid, ok := infix.Right.(*ast.InvalidExpression); !ok || invalid.Recovery.End.Type != lexer.COMMA {
		t.Fatalf("infix right operand = %#v, want comma-bounded invalid expression", infix.Right)
	}
	prefix, ok := array.Elements[2].(*ast.PrefixExpression)
	if !ok {
		t.Fatalf("third element = %T, want PrefixExpression", array.Elements[2])
	}
	if invalid, ok := prefix.Right.(*ast.InvalidExpression); !ok || invalid.Recovery.End.Type != lexer.COMMA {
		t.Fatalf("prefix right operand = %#v, want comma-bounded invalid expression", prefix.Right)
	}
	if array.Elements[1].String() != "2" || array.Elements[3].String() != "4" {
		t.Fatalf("valid sibling elements were not preserved: %#v", array.Elements)
	}
}
