package sema

import "sec/internal/ast"

// inferPlainArithmeticExpression checks ordinary numeric compatibility and
// contextual literals, retaining the existing text-concatenation and explicit
// conversion rules. Rejected operand pairs use the shared operator diagnostic.
// Rules: rules/foundations/operators.md — §§16–18;
// rules/types/types.md — No implicit conversion;
// rules/errors/runtime_checks.md — Arithmetic checks.
func (a *Analyzer) inferPlainArithmeticExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if expr.Operator == "+" && (isConcatTextual(leftType) || isConcatTextual(rightType)) {
		if !isConcatOperand(leftType) || !isConcatOperand(rightType) {
			a.addInvalidConcatOperandError(expr.Token, leftType, rightType)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.recordStringConcatPlan(expr, expr.Left, expr.Right)
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	}

	if !isNumericType(leftType) || !isNumericType(rightType) {
		help := numericCarrierOperandHelp(leftType, rightType, expr.Left, expr.Right)
		if help == "" {
			help = referenceOperandHelp(leftType, rightType, expr.Left, expr.Right)
		}
		if help != "" {
			a.addErrorAtTokenWithMetadata(expr.Token, "", help, "operator %s requires numeric operands", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		if isCoreDurationType(leftType) || isCoreDurationType(rightType) {
			// Owner decision 2026-11-08: duration has comparisons only.
			a.addErrorAtTokenWithMetadata(expr.Token, "", "duration supports ==, !=, <, <=, > and >= only; arithmetic on durations is not part of Sec 0.1.", "operator %s is not defined for duration", expr.Operator)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(expr.Token, "operator %s requires numeric operands", expr.Operator)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	var valid bool
	rightType, valid = a.contextualNumericLiteralType(expr.Right, rightType, leftType)
	if !valid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	leftType, valid = a.contextualNumericLiteralType(expr.Left, leftType, rightType)
	if !valid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if sameConcreteType(leftType, rightType) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if compatiblePlainNumericAlias(leftType, rightType) {
		if leftType.Named {
			if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
			}
			return rightType, expressionValue{Display: expr.String()}
		}
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Right) && a.canInitialize(leftType, rightType, expr.Right) {
		if !a.validateCompileTimeIntegerArithmetic(expr, leftType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return leftType, expressionValue{Display: expr.String()}
	}

	if isNumericLiteral(expr.Left) && a.canInitialize(rightType, leftType, expr.Left) {
		if !a.validateCompileTimeIntegerArithmetic(expr, rightType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return rightType, expressionValue{Display: expr.String()}
	}

	a.addOperatorOperandTypeError(expr, leftType, rightType)
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}
