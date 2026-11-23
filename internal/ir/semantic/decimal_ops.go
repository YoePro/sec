package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Decimal operators in Semantic IR (P21 §§ 16–20, 25–28; MD-066 §§ 3–6,
// MD-067 § 7). Each operation carries Sema's immutable plan: coefficient
// width, preferred-scale rule, nearest-exact scale selection, nearest-even
// rounding of an inexact quotient, truncating remainder and the
// ArithmeticError variants it can produce. Checked operations share the
// (result, failed, reason) CFG of checked integer arithmetic; a rounded
// quotient is a success, never a failure.
//
// Rules:
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3–7, 12(3)–12(4)
//   - rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md
//   - rules/foundations/operators.md — "Decimal arithmetic", "Decimal division"

const (
	OpDecimalNegChecked    OpKind = "decimal.neg-checked"
	OpDecimalBinaryChecked OpKind = "decimal.binary-checked"
	OpDecimalCompare       OpKind = "decimal.compare"
)

// DecimalBinaryKind is the arithmetic of decimal.binary-checked.
type DecimalBinaryKind string

const (
	DecimalBinaryAdd       DecimalBinaryKind = "add"
	DecimalBinarySubtract  DecimalBinaryKind = "sub"
	DecimalBinaryMultiply  DecimalBinaryKind = "mul"
	DecimalBinaryDivide    DecimalBinaryKind = "div"
	DecimalBinaryRemainder DecimalBinaryKind = "rem"
)

// DecimalOperationFacts is the detached Sema plan of one decimal operation.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — § 12(3).
type DecimalOperationFacts struct {
	Kind            DecimalBinaryKind
	CoefficientBits int
	// PreferredScale is max-operand-scale, sum-of-operand-scales or
	// operand-scale (MD-066 § 3(2)).
	PreferredScale string
	// RoundsInexactQuotient: division rounds once with nearest-even at the
	// finest admissible scale and succeeds (MD-066 § 5).
	RoundsInexactQuotient bool
	// TruncatingRemainder: q = trunc(a / b), r = a - q*b exactly (§ 4(2)).
	TruncatingRemainder bool
	// Failures are the ArithmeticError reasons the operation can produce.
	Failures []ArithmeticFailureReason
}

var decimalBinaryKinds = map[sema.ResolvedOperatorKind]DecimalBinaryKind{
	sema.ResolvedDecimalAddChecked:       DecimalBinaryAdd,
	sema.ResolvedDecimalSubtractChecked:  DecimalBinarySubtract,
	sema.ResolvedDecimalMultiplyChecked:  DecimalBinaryMultiply,
	sema.ResolvedDecimalDivideChecked:    DecimalBinaryDivide,
	sema.ResolvedDecimalRemainderChecked: DecimalBinaryRemainder,
}

var decimalComparePredicates = map[sema.ResolvedOperatorKind]IntegerComparePredicate{
	sema.ResolvedDecimalCompareEQ: IntegerCompareEQ, sema.ResolvedDecimalCompareNE: IntegerCompareNE,
	sema.ResolvedDecimalCompareLT: IntegerCompareLT, sema.ResolvedDecimalCompareLE: IntegerCompareLE,
	sema.ResolvedDecimalCompareGT: IntegerCompareGT, sema.ResolvedDecimalCompareGE: IntegerCompareGE,
}

// decimalFailureReasons maps Sema's ArithmeticError variant names to reasons.
var decimalFailureReasons = map[string]ArithmeticFailureReason{
	"Overflow": ArithmeticFailureReasonOverflow, "DivisionByZero": ArithmeticFailureDivisionByZero,
	"ScaleOverflow": ArithmeticFailureScaleOverflow, "PrecisionExhausted": ArithmeticFailurePrecisionExhausted,
}

// decimalOperatorKind selects the decimal Semantic IR operation of a
// Sema-resolved decimal operator and copies its plan.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — § 12(3).
func decimalOperatorKind(resolved sema.ResolvedOperator, op *Operation) bool {
	if resolved.Decimal == nil {
		return false
	}
	if predicate, ok := decimalComparePredicates[resolved.Kind]; ok {
		op.Kind, op.IntegerCompare = OpDecimalCompare, predicate
		return true
	}
	facts := &DecimalOperationFacts{
		CoefficientBits:       resolved.Decimal.CoefficientBits,
		PreferredScale:        string(resolved.Decimal.PreferredScale),
		RoundsInexactQuotient: resolved.Decimal.RoundsInexactQuotient,
		TruncatingRemainder:   resolved.Decimal.TruncatingRemainder,
	}
	for _, variant := range resolved.Decimal.PossibleFailures {
		reason, ok := decimalFailureReasons[variant]
		if !ok {
			return false
		}
		facts.Failures = append(facts.Failures, reason)
	}
	if resolved.Kind == sema.ResolvedDecimalNegateChecked {
		op.Kind, op.DecimalOperation = OpDecimalNegChecked, facts
		return true
	}
	kind, ok := decimalBinaryKinds[resolved.Kind]
	if !ok {
		return false
	}
	facts.Kind = kind
	op.Kind, op.DecimalOperation = OpDecimalBinaryChecked, facts
	return true
}

// decimalOperatorBoundary admits decimal operators over the primitive
// carriers only and materializes Sema's folded exact result. A decimal mixed
// with an integer operand needs an exact integer-to-decimal conversion first,
// and named/fixed-scale decimals keep their scale contracts (MD-049); both
// stay named boundaries.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3, 12(3);
// rules/compiler/semantic_ir.md — "Unsupported lowerings".
func (fb *functionBuilder) decimalOperatorBoundary(expr ast.Expression, resolved sema.ResolvedOperator, leftType, resultType TypeID) (builtValue, bool, error) {
	types := fb.owner.module.Types
	operandsPlain := isPlainDecimalType(types, leftType)
	if resolved.RightType != nil {
		rightType, err := fb.owner.internType(*resolved.RightType)
		if err != nil {
			return builtValue{}, true, err
		}
		operandsPlain = operandsPlain && rightType == leftType
	}
	if !operandsPlain {
		return builtValue{}, true, fb.unsupported("decimal operator with an integer, mixed-width or named-decimal operand", expressionToken(expr))
	}
	if constant := resolved.Decimal.Constant; constant != nil && isPlainDecimalType(types, resultType) {
		value := sema.DefaultConstant{Decimal: &sema.DecimalValue{Coefficient: constant.Coefficient, Scale: constant.Scale}, Lexeme: expr.String()}
		decimal, err := fb.decimalFromSemaConstant(value, resultType)
		if err != nil {
			return builtValue{}, true, err
		}
		return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: locationFromExpression(expr)}, resultType), true, nil
	}
	return builtValue{}, false, nil
}

// formatDecimalOperation prints the kind, operands and plan facts.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — § 12(4).
func formatDecimalOperation(out *strings.Builder, op Operation) {
	facts := op.DecimalOperation
	if facts == nil {
		out.WriteString(" <missing decimal plan>")
		return
	}
	if facts.Kind != "" {
		fmt.Fprintf(out, " %s", facts.Kind)
	}
	for index, operand := range op.Operands {
		if index > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, " %%%d", operand)
	}
	fmt.Fprintf(out, " {coefficient-bits=%d, preferred-scale=%s", facts.CoefficientBits, facts.PreferredScale)
	if facts.RoundsInexactQuotient {
		out.WriteString(", quotient=nearest-even")
	}
	if facts.TruncatingRemainder {
		out.WriteString(", remainder=truncating")
	}
	out.WriteString(", failures=[")
	for index, reason := range facts.Failures {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString(string(reason))
	}
	out.WriteString("]}")
}

// isPlainDecimalType admits the primitive decimal carriers; named and
// fixed-scale decimals keep their own scale/contract obligations (MD-049).
// Rules: rules/memory/layout.md — § 19.
func isPlainDecimalType(types *TypeTable, id TypeID) bool {
	return typeHasKind(types, id, TypeDecimal, TypeDecimal128)
}

// decimalCoefficientBitsOf is the coefficient width of a primitive decimal.
// Rules: rules/memory/layout.md — § 19(1).
func decimalCoefficientBitsOf(types *TypeTable, id TypeID) int {
	if typeHasKind(types, id, TypeDecimal128) {
		return 128
	}
	return 64
}

// verifyDecimalOperation checks operand/result types, the plan's preferred
// scale rule, division rounding and remainder flags, and the failure set.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3(2), 5, 7(3), 12(4).
func verifyDecimalOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if op.Kind == OpDecimalCompare {
		if !validIntegerComparePredicate(op.IntegerCompare) || len(op.Operands) != 2 || len(op.Results) != 1 ||
			!isPlainDecimalType(types, values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type ||
			!typeHasKind(types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid decimal.compare")
		}
		return nil
	}
	operands := 2
	if op.Kind == OpDecimalNegChecked {
		operands = 1
	}
	facts := op.DecimalOperation
	if facts == nil || len(op.Operands) != operands || len(op.Results) != 3 {
		return fmt.Errorf("invalid %s", op.Kind)
	}
	operandType := values[op.Operands[0]].Type
	if !isPlainDecimalType(types, operandType) || op.Results[0].Type != operandType ||
		(operands == 2 && values[op.Operands[1]].Type != operandType) ||
		!typeHasKind(types, op.Results[1].Type, TypeBool) || !isFailureReasonType(types, op.Results[2].Type) {
		return fmt.Errorf("invalid %s operand or result types", op.Kind)
	}
	if facts.CoefficientBits != decimalCoefficientBitsOf(types, operandType) {
		return fmt.Errorf("%s coefficient width %d does not match its type", op.Kind, facts.CoefficientBits)
	}
	wantScale := string(sema.DecimalPreferredMaxScale)
	switch {
	case op.Kind == OpDecimalNegChecked:
		wantScale = string(sema.DecimalPreferredOperandScale)
	case facts.Kind == DecimalBinaryMultiply:
		wantScale = string(sema.DecimalPreferredSumScale)
	}
	if op.Kind == OpDecimalBinaryChecked {
		switch facts.Kind {
		case DecimalBinaryAdd, DecimalBinarySubtract, DecimalBinaryMultiply, DecimalBinaryDivide, DecimalBinaryRemainder:
		default:
			return fmt.Errorf("invalid decimal.binary-checked kind %q", facts.Kind)
		}
	}
	if facts.PreferredScale != wantScale {
		return fmt.Errorf("%s preferred scale %q, want %q", op.Kind, facts.PreferredScale, wantScale)
	}
	if facts.RoundsInexactQuotient != (facts.Kind == DecimalBinaryDivide) || facts.TruncatingRemainder != (facts.Kind == DecimalBinaryRemainder) {
		return fmt.Errorf("%s rounding/remainder policy does not match kind %q", op.Kind, facts.Kind)
	}
	divides := facts.Kind == DecimalBinaryDivide || facts.Kind == DecimalBinaryRemainder
	seenZero := false
	for _, reason := range facts.Failures {
		switch reason {
		case ArithmeticFailureScaleOverflow, ArithmeticFailurePrecisionExhausted:
		case ArithmeticFailureDivisionByZero:
			seenZero = true
			if !divides {
				return fmt.Errorf("%s cannot divide by zero", op.Kind)
			}
		case ArithmeticFailureReasonOverflow:
			if facts.Kind != DecimalBinaryDivide {
				return fmt.Errorf("%s: only division reports generic Overflow", op.Kind)
			}
		default:
			return fmt.Errorf("%s has invalid failure reason %q", op.Kind, reason)
		}
	}
	if divides && !seenZero {
		return fmt.Errorf("%s must list division-by-zero", op.Kind)
	}
	return nil
}
