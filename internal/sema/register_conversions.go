package sema

import (
	"math/big"

	"sec/internal/ast"
)

// Detached register bit-pattern conversions (MD-110; registers.md § 12.3).
// `T(x)` builds the exact N-bit pattern of a primitive integer when x is
// bit-pattern representable: 0 <= x <= 2^N-1, or -2^(N-1) <= x < 0 for a
// signed source, taking the low N two's-complement bits. A constant that does
// not fit is a compile-time error; a runtime source whose whole domain fits
// (or a bit[N](x) of at most N bits) converts exactly, and any other source
// is the checked conversion. `IntegerType(r)` reads the bits zero- or
// sign-extended into an integer of width W >= N and is total.

// RegisterConversionKind is the direction of a register conversion.
type RegisterConversionKind string

const (
	RegisterFromBits RegisterConversionKind = "from-bits"
	RegisterToBits   RegisterConversionKind = "to-bits"
)

// RegisterFitBasis is how an exact integer-to-register conversion is proven
// (MD-110 § 7(3)); RegisterFitGuarded marks the checked form, whose runtime
// fits-bits predicate guards the conversion.
type RegisterFitBasis string

const (
	RegisterFitConstant      RegisterFitBasis = "constant"
	RegisterFitSourceDomain  RegisterFitBasis = "source-domain"
	RegisterFitBitExtraction RegisterFitBasis = "bit-extraction"
	RegisterFitGuarded       RegisterFitBasis = "guarded"
)

// ResolvedRegisterConversion is Sema's resolution of one explicit register
// conversion, keyed by its operand expression.
type ResolvedRegisterConversion struct {
	Kind     RegisterConversionKind
	Register Type
	Integer  Type
	// Signed is the integer endpoint's interpretation.
	Signed bool
	// Basis proves a from-bits conversion; Constant is the operand value
	// when it is a compile-time constant.
	Basis    RegisterFitBasis
	Constant *big.Int
}

// ResolvedRegisterConversionOf returns the resolved conversion whose operand
// is expr.
func (a *Analyzer) ResolvedRegisterConversionOf(operand ast.Expression) (ResolvedRegisterConversion, bool) {
	if a == nil || operand == nil {
		return ResolvedRegisterConversion{}, false
	}
	conversion, ok := a.resolvedRegisterConversions[operand]
	if ok && conversion.Constant != nil {
		conversion.Constant = new(big.Int).Set(conversion.Constant)
	}
	return conversion, ok
}

func (a *Analyzer) recordRegisterConversion(operand ast.Expression, conversion ResolvedRegisterConversion) {
	if a.resolvedRegisterConversions == nil {
		a.resolvedRegisterConversions = map[ast.Expression]ResolvedRegisterConversion{}
	}
	a.resolvedRegisterConversions[operand] = conversion
}

// registerBitPatternFits is the MD-110 § 3(2) fit rule for one value.
func registerBitPatternFits(value *big.Int, width int64) bool {
	if value.Sign() >= 0 {
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
		return value.Cmp(maximum) <= 0
	}
	minimum := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(width-1)))
	return value.Cmp(minimum) >= 0
}

// integerToRegisterConversion resolves `T(x)`, returning T for an exact
// conversion, Result[T, ArithmeticError] for the checked form, or invalid.
func (a *Analyzer) integerToRegisterConversion(target, source Type, operand ast.Expression) Type {
	token := expressionToken(operand)
	if source.Named || source.Declared || hasUnitSemantics(source) {
		a.addErrorAtTokenWithMetadata(token, "",
			"Convert the value to its primitive integer type first, then build the register from those bits.",
			"register %s is built from a primitive integer; %s keeps its own identity and needs an explicit conversion first", target.Name, typeDisplayName(source))
		return Type{Kind: InvalidType}
	}
	width := target.RegisterWidth
	conversion := ResolvedRegisterConversion{Kind: RegisterFromBits, Register: target, Integer: source, Signed: source.Kind == IntType}
	if value, constant := a.integerConstantValue(operand); constant {
		if !registerBitPatternFits(value, width) {
			a.addErrorAtTokenWithMetadata(token, "",
				"Use bit["+big.NewInt(width).String()+"](value) to keep only the low bits deliberately.",
				"integer value %s does not fit register %s: its bit pattern needs more than %d bits", value.String(), target.Name, width)
			return Type{Kind: InvalidType}
		}
		conversion.Basis, conversion.Constant = RegisterFitConstant, value
		a.recordRegisterConversion(operand, conversion)
		return target
	}
	if extraction, ok := operand.(*ast.BitExtractionExpression); ok {
		if _, high, proven := a.bitExtractionInterval(extraction); proven && registerBitPatternFits(high, width) {
			conversion.Basis = RegisterFitBitExtraction
			a.recordRegisterConversion(operand, conversion)
			return target
		}
	}
	if source.MinInteger != nil && source.MaxInteger != nil && registerBitPatternFits(source.MinInteger, width) && registerBitPatternFits(source.MaxInteger, width) {
		conversion.Basis = RegisterFitSourceDomain
		a.recordRegisterConversion(operand, conversion)
		return target
	}
	conversion.Basis = RegisterFitGuarded
	a.recordRegisterConversion(operand, conversion)
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{target, a.types["ArithmeticError"]}}
}

// registerToIntegerConversion resolves `IntegerType(r)` for a detached
// register: the target is a primitive integer at least as wide as the
// register.
func (a *Analyzer) registerToIntegerConversion(target, source Type, operand ast.Expression) Type {
	token := expressionToken(operand)
	if target.Named || target.Declared || hasUnitSemantics(target) {
		a.addErrorAtToken(token, "register %s converts only to a primitive integer type, not %s", source.Name, typeDisplayName(target))
		return Type{Kind: InvalidType}
	}
	width, resolved := integerRepresentationWidth(target)
	if !resolved {
		a.addErrorAtToken(token, "converting register %s to %s needs the target width of %s, which is unavailable without a selected target", source.Name, typeDisplayName(target), typeDisplayName(target))
		return Type{Kind: InvalidType}
	}
	if width < source.RegisterWidth {
		a.addErrorAtTokenWithMetadata(token, "",
			"Convert to an unsigned integer of at least "+big.NewInt(source.RegisterWidth).String()+" bits, then keep fewer bits with bit[M](value).",
			"%s is %d bits wide, narrower than register %s (%d bits); a register converts only to an integer of at least its width", typeDisplayName(target), width, source.Name, source.RegisterWidth)
		return Type{Kind: InvalidType}
	}
	a.recordRegisterConversion(operand, ResolvedRegisterConversion{Kind: RegisterToBits, Register: source, Integer: target, Signed: target.Kind == IntType})
	return target
}

// integerRepresentationWidth is a primitive integer's bit width, from its
// value domain; target-sized types are resolved only under a CompilationPlan.
func integerRepresentationWidth(typ Type) (int64, bool) {
	if typ.BitWidth > 0 {
		return typ.BitWidth, true
	}
	if typ.MaxInteger == nil || typ.MinInteger == nil {
		return 0, false
	}
	if typ.Kind == IntType {
		return int64(typ.MaxInteger.BitLen()) + 1, true
	}
	return int64(typ.MaxInteger.BitLen()), true
}
