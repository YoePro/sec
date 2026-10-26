package constant

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// FloatLiteralBits rounds exact source text directly into binary32/binary64,
// with ties to even and no wider intermediate. Integer-form g literals denote
// exact mathematical integers; source overflow is rejected, gradual underflow
// and signed zero are retained. Both CTE and Semantic IR use this boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§12–19,82.
func FloatLiteralBits(lexeme string, format uint16) (uint64, error) {
	if format != 32 && format != 64 {
		return 0, fmt.Errorf("float literal %s has no binary32/binary64 format", lexeme)
	}
	text := strings.ReplaceAll(lexeme, "_", "")
	text = strings.TrimSuffix(strings.TrimSuffix(text, "g"), "G")
	unsigned := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(text), "-"), "+")
	var number float64
	if strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0b") || strings.HasPrefix(unsigned, "0o") {
		integer, ok := new(big.Int).SetString(text, 0)
		if !ok {
			return 0, fmt.Errorf("invalid float literal %s", lexeme)
		}
		exact := new(big.Rat).SetInt(integer)
		if format == 32 {
			v, _ := exact.Float32()
			number = float64(v)
		} else {
			number, _ = exact.Float64()
		}
	} else {
		var err error
		number, err = strconv.ParseFloat(text, int(format))
		if err != nil {
			return 0, fmt.Errorf("float literal %s exceeds the finite range of float%d", lexeme, format)
		}
	}
	if math.IsInf(number, 0) || math.IsNaN(number) {
		return 0, fmt.Errorf("float literal %s exceeds the finite range of float%d", lexeme, format)
	}
	if number == 0 && strings.HasPrefix(text, "-") {
		number = math.Copysign(0, -1)
	}
	if format == 32 {
		return uint64(math.Float32bits(float32(number))), nil
	}
	return math.Float64bits(number), nil
}
