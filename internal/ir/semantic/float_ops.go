package semantic

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Floating operations of Package 20 admitted early for the binary float
// subset: negation, the five arithmetic kinds and the six comparisons over
// one float type (float, float32, float64). Unary plus is an SSA identity.
// None fails at run time: overflow gives infinity and a zero divisor gives
// an infinity or NaN (package20.md §§ 20–26, 63–65).
const (
	OpFloatNeg     OpKind = "float.neg"
	OpFloatBinary  OpKind = "float.binary"
	OpFloatCompare OpKind = "float.compare"
)

// FloatBinaryKind is the arithmetic kind of float.binary; rem is the
// truncation remainder (package20.md § 23).
type FloatBinaryKind string

const (
	FloatBinaryAdd FloatBinaryKind = "add"
	FloatBinarySub FloatBinaryKind = "sub"
	FloatBinaryMul FloatBinaryKind = "mul"
	FloatBinaryDiv FloatBinaryKind = "div"
	FloatBinaryRem FloatBinaryKind = "rem"
)

func validFloatBinaryKind(kind FloatBinaryKind) bool {
	switch kind {
	case FloatBinaryAdd, FloatBinarySub, FloatBinaryMul, FloatBinaryDiv, FloatBinaryRem:
		return true
	}
	return false
}

// FloatConstant is the format-precise payload of const.float: the binary
// format and its canonical bit pattern, rounded once from the exact source
// value (package20.md §§ 12–19).
type FloatConstant struct {
	Bits      uint64
	Format    uint16 // 32 or 64
	Canonical bool
}

// floatLiteralBits rounds a source float literal directly into binary32 or
// binary64 with round-to-nearest, ties-to-even — never through a wider
// format. Base-prefixed integer forms (`0x10g`) denote the exact integer.
// A finite literal beyond the format's finite range is rejected; a tiny one
// becomes a subnormal or signed zero (package20.md §§ 13–17).
func floatLiteralBits(lexeme string, format uint16) (uint64, error) {
	text := strings.ReplaceAll(lexeme, "_", "")
	text = strings.TrimSuffix(strings.TrimSuffix(text, "g"), "G")
	if format != 32 && format != 64 {
		return 0, fmt.Errorf("float literal %s has no binary32/binary64 format", lexeme)
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0b") || strings.HasPrefix(lower, "0o") {
		integer, ok := new(big.Int).SetString(lower, 0)
		if !ok {
			return 0, fmt.Errorf("invalid float literal %s", lexeme)
		}
		precision := uint(53)
		if format == 32 {
			precision = 24
		}
		rounded := new(big.Float).SetPrec(precision).SetMode(big.ToNearestEven).SetInt(integer)
		if format == 32 {
			value, _ := rounded.Float32()
			if math.IsInf(float64(value), 0) {
				return 0, fmt.Errorf("float literal %s exceeds the finite range of float32", lexeme)
			}
			return uint64(math.Float32bits(value)), nil
		}
		value, _ := rounded.Float64()
		if math.IsInf(value, 0) {
			return 0, fmt.Errorf("float literal %s exceeds the finite range of float64", lexeme)
		}
		return math.Float64bits(value), nil
	}
	value, err := strconv.ParseFloat(text, int(format))
	if err != nil {
		return 0, fmt.Errorf("float literal %s exceeds the finite range of float%d", lexeme, format)
	}
	if format == 32 {
		return uint64(math.Float32bits(float32(value))), nil
	}
	return math.Float64bits(value), nil
}

// floatFormat is the binary format of a float type: float32 and float64 by
// name, and plain float by the module's plan width (MD-014).
func (b *builder) floatFormat(typeID TypeID) (uint16, bool) {
	typ, ok := b.module.Types.Lookup(typeID)
	if !ok || typ.Kind != TypeFloat {
		return 0, false
	}
	if typ.BitWidth == 32 || typ.BitWidth == 64 {
		return typ.BitWidth, true
	}
	if width := b.module.PointerWidthBits; width == 32 || width == 64 {
		return width, true
	}
	return 0, false
}

// buildFloatConstant interns a float literal with its canonical bits.
func (fb *functionBuilder) buildFloatConstant(lexeme string, typeID TypeID, loc Location) (builtValue, error) {
	format, ok := fb.owner.floatFormat(typeID)
	if !ok {
		return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: lexeme, Location: loc}, typeID), nil
	}
	bits, err := floatLiteralBits(lexeme, format)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: lexeme, Float: &FloatConstant{Bits: bits, Format: format, Canonical: true}, Location: loc}, typeID), nil
}

func verifyFloatOperation(module *Module, op Operation, values map[ValueID]Value) error {
	isFloat := func(id TypeID) bool { return typeHasKind(module.Types, id, TypeFloat) }
	switch op.Kind {
	case OpFloatNeg:
		if len(op.Operands) != 1 || len(op.Results) != 1 || !isFloat(values[op.Operands[0]].Type) || op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid float.neg")
		}
	case OpFloatBinary:
		if !validFloatBinaryKind(op.FloatBinary) || len(op.Operands) != 2 || len(op.Results) != 1 ||
			!isFloat(values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type ||
			op.Results[0].Type != values[op.Operands[0]].Type {
			return fmt.Errorf("invalid float.binary")
		}
	case OpFloatCompare:
		if !validIntegerComparePredicate(op.IntegerCompare) || len(op.Operands) != 2 || len(op.Results) != 1 ||
			!isFloat(values[op.Operands[0]].Type) || values[op.Operands[0]].Type != values[op.Operands[1]].Type ||
			!typeHasKind(module.Types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid float.compare (operand types %d and %d)", values[op.Operands[0]].Type, values[op.Operands[1]].Type)
		}
	}
	return nil
}

// buildShapedFloatSign lowers `-literal` / `+literal` whose untyped operand
// the float context shaped (package20.md § 17: `-0.0g` is positive-zero
// literal shaping followed by floating negation, giving negative zero).
func (fb *functionBuilder) buildShapedFloatSign(expr ast.Expression) (builtValue, bool, error) {
	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok || (prefix.Operator != "-" && prefix.Operator != "+") {
		return builtValue{}, false, nil
	}
	resolved, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok || resolved.Kind != sema.FloatType {
		return builtValue{}, false, nil
	}
	typeID, err := fb.owner.internType(resolved)
	if err != nil {
		return builtValue{}, true, err
	}
	operand, err := fb.buildExpr(prefix.Right, typeID)
	if err != nil {
		return builtValue{}, true, err
	}
	if operand.typ != typeID {
		return builtValue{}, true, fmt.Errorf("shaped float sign operand type mismatch")
	}
	if prefix.Operator == "+" {
		return operand, true, nil
	}
	return fb.result(Operation{Kind: OpFloatNeg, Operands: []ValueID{operand.id}, Location: location(prefix.Token)}, typeID), true, nil
}
