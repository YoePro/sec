package sema

import (
	"math/big"

	"sec/internal/ast"
)

// proveIntegerLiteralNegation discharges negation only for a builtin signed
// integer literal whose exact result fits the selected representation. It does
// not remove checks from runtime values, named contracts, decimal operators or
// other operations inside the protected expression.
// Rules: rules/errors/runtime_checks.md — §§5.1,7;
// rules/foundations/operators.md — §§18.1–18.2.
func (a *Analyzer) proveIntegerLiteralNegation(resolved ResolvedOperator, expr *ast.PrefixExpression) ResolvedOperator {
	if resolved.Kind != ResolvedIntegerNegateChecked || !isBuiltinIntegerOperatorType(resolved.LeftType) {
		return resolved
	}
	if _, literal := expr.Right.(*ast.IntegerLiteral); !literal {
		return resolved
	}
	value, known := a.integerConstantValue(expr.Right)
	representation, _, ok := a.integerRepresentation(a.types[resolved.LeftType.Name])
	if !known || !ok || representation.MinInteger == nil || representation.MaxInteger == nil {
		return resolved
	}
	negated := new(big.Int).Neg(value)
	if negated.Cmp(representation.MinInteger) < 0 || negated.Cmp(representation.MaxInteger) > 0 {
		return resolved
	}
	resolved.RuntimeCheck = false
	resolved.FailureBehavior = OperatorDoesNotFail
	resolved.NegatedIntegerConstant = negated.String()
	return resolved
}
