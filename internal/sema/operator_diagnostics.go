package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// addOperatorOperandTypeError gives an already-rejected binary operand pair a
// stable mandatory diagnostic, retaining the operator's exact span and existing
// message. Guidance requires an explicit compatible type rather than promising
// an implicit signed/unsigned conversion or an infallible cast.
// Rules: rules/tooling/diagnostics.md — §§5–9,12,15;
// rules/foundations/operators.md — §§16–17;
// rules/types/types.md — No implicit conversion.
func (a *Analyzer) addOperatorOperandTypeError(expr *ast.InfixExpression, left, right Type) {
	a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.OperatorIncompatibleOperands,
		"Use operands with compatible types. If a conversion is intended, choose the common type explicitly and handle any conversion failure; signed and unsigned integers are not implicitly interchangeable.",
		"cannot apply operator %s to %s and %s", expr.Operator, typeDisplayName(left), typeDisplayName(right))
}
