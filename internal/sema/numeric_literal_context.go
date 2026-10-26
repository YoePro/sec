package sema

import "sec/internal/ast"

// contextualNumericLiteralType publishes a numeric literal's expected type,
// including signed operands, without inserting a typed runtime conversion.
// Rules: rules/types/types.md — "Context shaping";
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–19.
func (a *Analyzer) contextualNumericLiteralType(expr ast.Expression, actual Type, target Type) (Type, bool) {
	if !isUntypedNumericExpression(expr) || !isNumericType(target) || !a.canInitialize(target, actual, expr) {
		return actual, true
	}
	if isIntegerType(target) {
		value, known := a.integerConstantValue(expr)
		if known && a.checkIntegerValueRange(target, value, expressionToken(expr)) {
			return Type{Kind: InvalidType}, false
		}
	}
	a.expressionTypes[expr] = target
	// A signed literal shapes its operand with it (`-1.5` beside a float64 is
	// the float64 literal 1.5 negated, package20.md § 17).
	if prefix, ok := expr.(*ast.PrefixExpression); ok && (prefix.Operator == "-" || prefix.Operator == "+") && target.Kind == FloatType {
		if operand, recorded := a.expressionTypes[prefix.Right]; recorded {
			a.contextualNumericLiteralType(prefix.Right, operand, target)
		}
		// A g literal may already have published a platform-float sign
		// operator. Refresh it after shaping both operand and result, so
		// downstream consumers cannot retain the earlier platform width.
		a.recordResolvedOperator(expr, target)
	}
	return target, true
}

// numericLiteralSource peels signs without losing an explicit family suffix.
// Rules: rules/types/types.md — "Numeric literal suffixes";
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–17.
func numericLiteralSource(expression ast.Expression) string {
	if prefix, ok := expression.(*ast.PrefixExpression); ok {
		return numericLiteralSource(prefix.Right)
	}
	return expression.TokenLiteral()
}
