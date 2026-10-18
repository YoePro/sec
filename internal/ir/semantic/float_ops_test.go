package semantic

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
)

// A float literal rounds once, directly into its binary format, with
// nearest-even; base-prefixed `g` forms are exact integers; out-of-range
// finite literals are rejected and tiny ones become zero or subnormal.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13–18
func TestFloatLiteralBitsRoundOnce(t *testing.T) {
	// The direct binary32 rounding of this value differs from rounding
	// through binary64 first (no double rounding, P20 § 14).
	tricky := "1.00000017881393432617187499"
	oracle, _, err := big.ParseFloat(tricky, 10, 24, big.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := oracle.Float32()
	viaDouble, _ := strconv.ParseFloat(tricky, 64)
	if math.Float32bits(float32(viaDouble)) == math.Float32bits(want) {
		t.Fatal("test literal does not exercise double rounding")
	}
	for _, test := range []struct {
		lexeme string
		format uint16
		bits   uint64
	}{
		{"0.1", 32, 0x3dcccccd},
		{"0.1", 64, 0x3fb999999999999a},
		{"1.5g", 64, 0x3ff8000000000000},
		{"0x10g", 32, 0x41800000},
		{"0b101g", 64, 0x4014000000000000},
		{"1_000.5", 64, math.Float64bits(1000.5)},
		{"1e-50", 32, 0},
		{tricky, 32, uint64(math.Float32bits(want))},
	} {
		got, err := floatLiteralBits(test.lexeme, test.format)
		if err != nil || got != test.bits {
			t.Errorf("%s as binary%d: got %#x (%v), want %#x", test.lexeme, test.format, got, err, test.bits)
		}
	}
	for _, test := range []struct {
		lexeme string
		format uint16
	}{
		{"3.5e38", 32},
		{"1e400", 64},
		{"0x1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000g", 64},
	} {
		if _, err := floatLiteralBits(test.lexeme, test.format); err == nil || !strings.Contains(err.Error(), "finite range") {
			t.Errorf("%s accepted as binary%d: %v", test.lexeme, test.format, err)
		}
	}
}

// Floating operations verify their kind, predicate and one float type.
func TestFloatOperationVerifier(t *testing.T) {
	module := &Module{Types: NewTypeTable()}
	f64 := module.Types.Intern(Type{Kind: TypeFloat, Name: "float64", BitWidth: 64})
	f32 := module.Types.Intern(Type{Kind: TypeFloat, Name: "float32", BitWidth: 32})
	boolean := module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	values := map[ValueID]Value{1: {ID: 1, Type: f64}, 2: {ID: 2, Type: f64}, 3: {ID: 3, Type: f32}}
	valid := []Operation{
		{Kind: OpFloatNeg, Operands: []ValueID{1}, Results: []Value{{ID: 9, Type: f64}}},
		{Kind: OpFloatBinary, FloatBinary: FloatBinaryRem, Operands: []ValueID{1, 2}, Results: []Value{{ID: 9, Type: f64}}},
		{Kind: OpFloatCompare, IntegerCompare: IntegerCompareNE, Operands: []ValueID{1, 2}, Results: []Value{{ID: 9, Type: boolean}}},
	}
	for _, op := range valid {
		if err := verifyFloatOperation(module, op, values); err != nil {
			t.Errorf("%s rejected: %v", op.Kind, err)
		}
	}
	invalid := []Operation{
		{Kind: OpFloatBinary, FloatBinary: "pow", Operands: []ValueID{1, 2}, Results: []Value{{ID: 9, Type: f64}}},
		{Kind: OpFloatBinary, FloatBinary: FloatBinaryAdd, Operands: []ValueID{1, 3}, Results: []Value{{ID: 9, Type: f64}}},
		{Kind: OpFloatCompare, IntegerCompare: IntegerCompareLT, Operands: []ValueID{1, 2}, Results: []Value{{ID: 9, Type: f64}}},
		{Kind: OpFloatNeg, Operands: []ValueID{1}, Results: []Value{{ID: 9, Type: f32}}},
	}
	for _, op := range invalid {
		if err := verifyFloatOperation(module, op, values); err == nil {
			t.Errorf("accepted %+v", op)
		}
	}
}
