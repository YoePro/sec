package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/lexer"
	"sec/internal/sema"
	"sec/internal/sema/constant"
)

// fixedDecimalScale returns the fixed scale of a named decimal type.
// Rules: rules/compiler/semantic_ir.md — § 128(2).
func fixedDecimalScale(types *TypeTable, id TypeID) (*DecimalScaleFacts, bool) {
	typ, ok := types.Lookup(id)
	if !ok || typ.Kind != TypeNamed || typ.FixedScale == nil {
		return nil, false
	}
	return typ.FixedScale, true
}

// shapeDecimalToType places a decimal constant exactly at the fixed scale of
// its target type, multiplying the coefficient or removing exact trailing
// zeros; a value that would need rounding or a wider coefficient is rejected
// instead of being approximated. Other targets keep the constant unchanged.
// Rules: rules/compiler/semantic_ir.md — § 128(3); rules/types/types.md — § 45A(1);
// rules/types/default_values.md — § 65A(3).
func (fb *functionBuilder) shapeDecimalToType(decimal DecimalConstant, typeID TypeID) (DecimalConstant, error) {
	facts, fixed := fixedDecimalScale(fb.owner.module.Types, typeID)
	if !fixed || decimal.Coefficient == nil || decimal.Scale == facts.Scale {
		return decimal, nil
	}
	shift := int64(facts.Scale) - int64(decimal.Scale)
	coefficient := new(big.Int).Set(decimal.Coefficient)
	const maxShift = 1000
	if shift > maxShift || shift < -maxShift {
		return DecimalConstant{}, fb.unsupported(fmt.Sprintf("decimal constant rescale by %d places to fixed scale %d", shift, facts.Scale), lexer.Token{})
	}
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(abs64(shift)), nil)
	if shift > 0 {
		coefficient.Mul(coefficient, power)
	} else {
		quotient, remainder := new(big.Int).QuoRem(coefficient, power, new(big.Int))
		if remainder.Sign() != 0 {
			return DecimalConstant{}, fmt.Errorf("decimal %s cannot be represented exactly at fixed scale %d", decimal.Lexeme, facts.Scale)
		}
		coefficient = quotient
	}
	fitted, scale, ok := constant.FitDecimal(coefficient, int64(facts.Scale), fb.decimalBits(typeID))
	if !ok || scale != facts.Scale {
		return DecimalConstant{}, fmt.Errorf("decimal %s does not fit the coefficient width at fixed scale %d", decimal.Lexeme, facts.Scale)
	}
	return DecimalConstant{Coefficient: fitted, Scale: scale, Lexeme: decimal.Lexeme}, nil
}

// abs64 is the magnitude of a rescale shift.
// Rules: rules/compiler/semantic_ir.md — § 128(3).
func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

// decimalFromSemaConstant builds a decimal constant from Sema's
// scale-bearing pair, falling back to the source lexeme only for a value
// that has no pair; it never re-infers a computed value's scale from text.
// Rules: rules/compiler/semantic_ir.md — § 128(1)-(2);
// rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — § 13(5).
func (fb *functionBuilder) decimalFromSemaConstant(value sema.DefaultConstant, typeID TypeID) (DecimalConstant, error) {
	if value.Decimal != nil && value.Decimal.Coefficient != nil {
		decimal := DecimalConstant{Coefficient: new(big.Int).Set(value.Decimal.Coefficient), Scale: value.Decimal.Scale, Lexeme: value.Lexeme}
		if _, fixed := fixedDecimalScale(fb.owner.module.Types, typeID); fixed {
			return fb.shapeDecimalToType(decimal, typeID)
		}
		fitted, scale, ok := constant.FitDecimal(decimal.Coefficient, int64(decimal.Scale), fb.decimalBits(typeID))
		if !ok {
			return DecimalConstant{}, fmt.Errorf("decimal default %s has no exact %d-bit representation", value.Lexeme, fb.decimalBits(typeID))
		}
		decimal.Coefficient, decimal.Scale = fitted, scale
		return decimal, nil
	}
	decimal, err := parseDecimalFor(value.Lexeme, fb.decimalBits(typeID))
	if err != nil {
		return DecimalConstant{}, err
	}
	return fb.shapeDecimalToType(decimal, typeID)
}

// verifyFixedScaleDecimalConstant rejects a const.decimal of a fixed-scale
// named type whose scale differs from the type's frozen scale.
// Rules: rules/compiler/semantic_ir.md — § 128(3); rules/mlir/sec_mlir_dialect.md — § 44(2).
func verifyFixedScaleDecimalConstant(module *Module, op Operation) error {
	if op.Decimal == nil || len(op.Results) != 1 {
		return nil
	}
	if facts, fixed := fixedDecimalScale(module.Types, op.Results[0].Type); fixed && op.Decimal.Scale != facts.Scale {
		return fmt.Errorf("const.decimal of a fixed-scale type has scale %d, want %d", op.Decimal.Scale, facts.Scale)
	}
	return nil
}
