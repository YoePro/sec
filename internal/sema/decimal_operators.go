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
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3–7

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
	resultBase := dereferenceType(result)
	if scale, fixed := fixedDecimalScaleContract(resultBase); fixed && resultBase.Named {
		// MD-049 § 12(3)-(4), § 15(4): T op T is T, exact on the grid S.
		value := scale.Value
		plan.FixedScale = &value
		plan.RoundsInexactQuotient = false
		plan.ContractChecked = hasValueContracts(resultBase)
		plan.PossibleFailures = fixedScaleOperationFailures(expr.Operator)
		if leftValue, ok := a.decimalExactConstant(expr.Left); ok {
			if rightValue, ok := a.decimalExactConstant(expr.Right); ok {
				if value, failure, evaluated := fixedScaleDecimalConstant(expr.Operator, leftValue, rightValue, scale.Value, bits); evaluated {
					plan.Constant, plan.ConstantFailure = value, failure
				}
			}
		}
		resolved.Kind = kind
		resolved.RuntimeCheck = true
		resolved.FailureBehavior = OperatorArithmeticFailure
		resolved.Decimal = plan
		return resolved, true
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
	if resultBase := dereferenceType(result); resultBase.Named {
		if scale, fixed := fixedDecimalScaleContract(resultBase); fixed {
			// MD-049 § 12(3)-(4): -T stays on the grid S; only T's value
			// contracts can reject it.
			value := scale.Value
			plan.FixedScale = &value
			plan.ContractChecked = hasValueContracts(resultBase)
			plan.PossibleFailures = fixedScaleOperationFailures("unary-")
		}
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
		symbol, ok := a.symbols.lookup(node.Value)
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

// fixedScaleOperationFailures lists the intrinsic ArithmeticError variants of
// an operation producing a fixed-scale T: a result off the grid S, or with
// more digits than the coefficient at S, is PrecisionExhausted; the scale is
// fixed, so ScaleOverflow cannot arise; division and remainder also fail on
// a zero divisor. Contract failures are checked after these (MD-049 § 15(4)).
// Rules: rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — §§ 12(4), 15(4).
func fixedScaleOperationFailures(operator string) []string {
	if operator == "/" || operator == "%" {
		return []string{"DivisionByZero", "PrecisionExhausted"}
	}
	return []string{"PrecisionExhausted"}
}

// fixedScaleDecimalConstant evaluates a constant T op T exactly as a
// rational, then places it on the grid S without rounding: a nonintegral
// coefficient at S or one outside the coefficient width fails with
// PrecisionExhausted, a zero divisor with DivisionByZero. Remainder is the
// exact truncating remainder. The result keeps scale S, also for zero.
// Rules: rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — §§ 12(4), 12(8), 15(4).
func fixedScaleDecimalConstant(operator string, left, right DecimalValue, scale int32, bits int) (*DecimalValue, string, bool) {
	const maxShift = 1000
	if abs := int64(scale); abs > maxShift || abs < -maxShift || int64(left.Scale) > maxShift || int64(left.Scale) < -maxShift || int64(right.Scale) > maxShift || int64(right.Scale) < -maxShift {
		return nil, "", false
	}
	rational := func(value DecimalValue) *big.Rat {
		r := new(big.Rat).SetInt(value.Coefficient)
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(abs64Sema(int64(value.Scale))), nil)
		if value.Scale >= 0 {
			return r.Quo(r, new(big.Rat).SetInt(power))
		}
		return r.Mul(r, new(big.Rat).SetInt(power))
	}
	a, b := rational(left), rational(right)
	var exact *big.Rat
	switch operator {
	case "+":
		exact = new(big.Rat).Add(a, b)
	case "-":
		exact = new(big.Rat).Sub(a, b)
	case "*":
		exact = new(big.Rat).Mul(a, b)
	case "/", "%":
		if b.Sign() == 0 {
			return nil, "DivisionByZero", true
		}
		exact = new(big.Rat).Quo(a, b)
		if operator == "%" {
			quotient := new(big.Int).Quo(exact.Num(), exact.Denom())
			exact = new(big.Rat).Sub(a, new(big.Rat).Mul(new(big.Rat).SetInt(quotient), b))
		}
	default:
		return nil, "", false
	}
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(abs64Sema(int64(scale))), nil)
	grid := new(big.Rat).Set(exact)
	if scale >= 0 {
		grid.Mul(grid, new(big.Rat).SetInt(power))
	} else {
		grid.Quo(grid, new(big.Rat).SetInt(power))
	}
	if !grid.IsInt() {
		return nil, "PrecisionExhausted", true
	}
	coefficient := new(big.Int).Set(grid.Num())
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	if coefficient.Cmp(limit) >= 0 || coefficient.Cmp(new(big.Int).Neg(limit)) < 0 {
		return nil, "PrecisionExhausted", true
	}
	return &DecimalValue{Coefficient: coefficient, Scale: scale}, "", true
}

// abs64Sema is the magnitude of a scale shift.
// Rules: rules/types/types.md — "Decimal scale".
func abs64Sema(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

// hasValueContracts reports contracts other than the fixed scale itself.
// Rules: rules/types/contracts.md — §§ 12–16.
func hasValueContracts(typ Type) bool {
	for _, contract := range typ.Contracts {
		if _, scale := contract.(ScaleContract); !scale {
			return true
		}
	}
	return false
}
