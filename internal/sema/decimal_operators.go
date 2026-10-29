package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema/constant"
)

// Decimal operator plans (MD-066 §§ 3–6, MD-067 § 7). Decimal arithmetic is a
// checked runtime operation: an ordinary failure reaches the panic endpoint
// and `try` turns it into ArithmeticError. The plan records the preferred
// scale rule, rounding and remainder policy, the variants each operation can
// produce, and the exact result of constant operands.
//
// Rules:
//   - rules/foundations/operators.md — "Decimal arithmetic", "Decimal result scale", "Decimal division"
//   - rules/errors/runtime_checks.md — "Division and remainder"
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 3–7

// decimalOperationFailures lists, per operator, the ArithmeticError variants
// a decimal operation can produce (MD-067 § 7(3)). Exact operations fail
// only when no legal coefficient/scale pair holds the exact result; division
// also fails on a zero divisor or an unrepresentable whole-number part.
var decimalOperationFailures = map[string][]string{
	"+":      {"ScaleOverflow", "PrecisionExhausted"},
	"-":      {"ScaleOverflow", "PrecisionExhausted"},
	"*":      {"ScaleOverflow", "PrecisionExhausted"},
	"/":      {"DivisionByZero", "ScaleOverflow", "PrecisionExhausted", "Overflow"},
	"%":      {"DivisionByZero", "ScaleOverflow", "PrecisionExhausted"},
	"unary-": {"ScaleOverflow", "PrecisionExhausted"},
}

// resolvedDecimalInfixOperator resolves a decimal arithmetic or comparison
// operator whose operands are decimal, or a decimal combined with an
// integer (whose scale is 0).
func (a *Analyzer) resolvedDecimalInfixOperator(expr *ast.InfixExpression, left, right, result Type) (ResolvedOperator, bool) {
	leftBase, rightBase := dereferenceType(left), dereferenceType(right)
	if leftBase.Kind != DecimalType && rightBase.Kind != DecimalType {
		return ResolvedOperator{}, false
	}
	decimalSide := leftBase
	if decimalSide.Kind != DecimalType {
		decimalSide = rightBase
	}
	bits := int(decimalCoefficientBits(decimalSide))
	resolved := ResolvedOperator{LeftType: left, RightType: &right, ResultType: result, FailureBehavior: OperatorDoesNotFail}
	if kind, ok := map[string]ResolvedOperatorKind{
		"==": ResolvedDecimalCompareEQ, "!=": ResolvedDecimalCompareNE,
		"<": ResolvedDecimalCompareLT, "<=": ResolvedDecimalCompareLE,
		">": ResolvedDecimalCompareGT, ">=": ResolvedDecimalCompareGE,
	}[expr.Operator]; ok {
		if leftBase.Kind != DecimalType || rightBase.Kind != DecimalType {
			return ResolvedOperator{}, false
		}
		resolved.Kind = kind
		resolved.Decimal = &ResolvedDecimalOperation{CoefficientBits: bits, ScaleIndependentComparison: true}
		return resolved, true
	}
	kind, ok := map[string]ResolvedOperatorKind{
		"+": ResolvedDecimalAddChecked, "-": ResolvedDecimalSubtractChecked,
		"*": ResolvedDecimalMultiplyChecked, "/": ResolvedDecimalDivideChecked,
		"%": ResolvedDecimalRemainderChecked,
	}[expr.Operator]
	if !ok || dereferenceType(result).Kind != DecimalType {
		return ResolvedOperator{}, false
	}
	plan := &ResolvedDecimalOperation{
		CoefficientBits:       bits,
		PreferredScale:        DecimalPreferredMaxScale,
		RoundsInexactQuotient: expr.Operator == "/",
		TruncatingRemainder:   expr.Operator == "%",
		PossibleFailures:      append([]string(nil), decimalOperationFailures[expr.Operator]...),
	}
	if expr.Operator == "*" {
		plan.PreferredScale = DecimalPreferredSumScale
	}
	if leftValue, ok := a.decimalExactConstant(expr.Left); ok {
		if rightValue, ok := a.decimalExactConstant(expr.Right); ok {
			coefficient, scale, failure, evaluated := constant.EvaluateDecimal(expr.Operator, leftValue.Coefficient, int64(leftValue.Scale), rightValue.Coefficient, int64(rightValue.Scale), bits)
			if evaluated {
				if failure != "" {
					plan.ConstantFailure = string(failure)
				} else {
					plan.Constant = &DecimalValue{Coefficient: coefficient, Scale: int32(scale)}
				}
			}
		}
	}
	resolved.Kind = kind
	resolved.RuntimeCheck = true
	resolved.FailureBehavior = OperatorArithmeticFailure
	resolved.Decimal = plan
	return resolved, true
}

// resolvedDecimalNegate resolves unary minus on a decimal operand; negating
// the most negative coefficient needs one more digit.
func (a *Analyzer) resolvedDecimalNegate(expr *ast.PrefixExpression, operand, result Type) ResolvedOperator {
	bits := int(decimalCoefficientBits(dereferenceType(operand)))
	plan := &ResolvedDecimalOperation{
		CoefficientBits:  bits,
		PreferredScale:   DecimalPreferredOperandScale,
		PossibleFailures: append([]string(nil), decimalOperationFailures["unary-"]...),
	}
	if value, ok := a.decimalExactConstant(expr.Right); ok {
		if coefficient, scale, failure, evaluated := constant.NegateDecimal(value.Coefficient, int64(value.Scale), bits); evaluated {
			if failure != "" {
				plan.ConstantFailure = string(failure)
			} else {
				plan.Constant = &DecimalValue{Coefficient: coefficient, Scale: int32(scale)}
			}
		}
	}
	return ResolvedOperator{
		Kind: ResolvedDecimalNegateChecked, LeftType: operand, ResultType: result,
		RuntimeCheck: true, FailureBehavior: OperatorArithmeticFailure, Decimal: plan,
	}
}

// decimalExactConstant returns the exact value and scale of a constant
// decimal expression: a decimal or integer literal (an integer has scale 0),
// a signed literal, an immutable binding with a folded value, or a folded
// decimal operation.
func (a *Analyzer) decimalExactConstant(expr ast.Expression) (DecimalValue, bool) {
	switch node := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		return decimalLiteralValue(node)
	case *ast.Identifier:
		symbol, ok := a.symbols[node.Value]
		if !ok || symbol.Mutable || symbol.Volatile || symbol.DecimalExact == nil {
			return DecimalValue{}, false
		}
		return DecimalValue{Coefficient: new(big.Int).Set(symbol.DecimalExact.Coefficient), Scale: symbol.DecimalExact.Scale}, true
	case *ast.PrefixExpression:
		if node.Operator == "+" {
			return a.decimalExactConstant(node.Right)
		}
		if literal, ok := decimalLiteralValue(node); ok {
			return literal, true
		}
	}
	if resolved, ok := a.resolvedOperators[expr]; ok && resolved.Decimal != nil && resolved.Decimal.Constant != nil {
		constant := resolved.Decimal.Constant
		return DecimalValue{Coefficient: new(big.Int).Set(constant.Coefficient), Scale: constant.Scale}, true
	}
	return DecimalValue{}, false
}
