package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestParseOwnershipAvailabilityConditions retains positive and negated
// ownership tests as dedicated AST rather than Option/null comparisons.
//
// Rules:
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/corrections/applied/correction30-20260828.md — §§1–3
func TestParseOwnershipAvailabilityConditions(t *testing.T) {
	p := New(lexer.New(`module main
fn Check(value: int) void {
    if value is available {}
    if value is not available {}
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	function := program.Statements[1].(*ast.FunctionDeclaration)
	positive := function.Body.Statements[0].(*ast.IfStatement).Condition.(*ast.AvailabilityExpression)
	negative := function.Body.Statements[1].(*ast.IfStatement).Condition.(*ast.AvailabilityExpression)
	if positive.Negated || !negative.Negated {
		t.Fatalf("availability polarity lost: positive=%#v negative=%#v", positive, negative)
	}
	if place, ok := positive.Place.(*ast.Identifier); !ok || place.Value != "value" {
		t.Fatalf("availability Place = %#v", positive.Place)
	}
}
