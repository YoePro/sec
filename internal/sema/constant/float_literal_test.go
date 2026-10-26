package constant

import "testing"

// Exact IEEE witnesses distinguish direct target rounding from a float64
// intermediate and preserve sign even for integer-form negative zero.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§13–19,82.
func TestFloatLiteralCanonicalBits(t *testing.T) {
	for _, test := range []struct {
		text   string
		format uint16
		bits   uint64
	}{
		{"1.00000017881393432617187499", 32, 0x3f800001},
		{"0x100000100000001g", 32, 0x5b800001},
		{"-0x0g", 32, 0x80000000},
		{"-0b0g", 64, 0x8000000000000000},
		{"-1e-400g", 64, 0x8000000000000000},
		{"1.401298464324817e-45g", 32, 1},
		{"4.9406564584124654e-324g", 64, 1},
	} {
		got, err := FloatLiteralBits(test.text, test.format)
		if err != nil || got != test.bits {
			t.Errorf("%s binary%d: %#x, %v; want %#x", test.text, test.format, got, err, test.bits)
		}
	}
	for _, text := range []string{"3.5e38", "1e400g", "NaN", "Inf", "0xNOg"} {
		if _, err := FloatLiteralBits(text, 32); err == nil {
			t.Errorf("invalid literal accepted: %s", text)
		}
	}
	for _, format := range []uint16{0, 16, 80} {
		if _, err := FloatLiteralBits("1", format); err == nil {
			t.Errorf("invalid format accepted: %d", format)
		}
	}
}
