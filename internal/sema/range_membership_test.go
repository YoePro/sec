package sema

import (
	"testing"

	"sec/internal/ast"
)

// Range membership publishes its plan (negation, bounds, exclusivity, value
// type) and gives fitting untyped literal bounds the value's type; `!` on bool
// publishes a non-failing bool-not operator.
//
// Rules:
//   - rules/foundations/operators.md — "Range membership", "Inclusive range", "Exclusive upper range", "Logical operators"
func TestRangeMembershipAndBoolNotPublishResolvedFacts(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
fn Small(value: uint8) bool {
    return value not in 1..<10
}

fn Flip(ready: bool) bool {
    return !ready
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	var membership *ast.InfixExpression
	var negation *ast.PrefixExpression
	for expr := range analyzer.expressionTypes {
		switch expr := expr.(type) {
		case *ast.InfixExpression:
			if expr.Operator == "not in" {
				membership = expr
			}
		case *ast.PrefixExpression:
			if expr.Operator == "!" {
				negation = expr
			}
		}
	}
	if membership == nil || negation == nil {
		t.Fatal("expressions not found")
	}
	plan, ok := analyzer.ResolvedRangeMembershipOf(membership)
	if !ok || !plan.Negated || !plan.HasStart || !plan.HasEnd || !plan.Exclusive || plan.ValueType.Name != "uint8" {
		t.Fatalf("plan = %+v, %v", plan, ok)
	}
	rangeExpr := membership.Right.(*ast.RangeExpression)
	for _, bound := range []ast.Expression{rangeExpr.Start, rangeExpr.End} {
		if typ, _ := analyzer.ResolvedTypeOf(bound); typ.Name != "uint8" {
			t.Errorf("literal bound type = %s, want uint8", typ.Name)
		}
	}
	resolved, ok := analyzer.ResolvedOperatorOf(negation)
	if !ok || resolved.Kind != ResolvedBoolNot || resolved.RuntimeCheck || resolved.FailureBehavior != OperatorDoesNotFail {
		t.Fatalf("! = %+v, %v", resolved, ok)
	}
}
