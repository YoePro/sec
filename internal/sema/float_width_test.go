package sema

import (
	"math/big"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Plain float is platform-sized exactly like int and uint: float32 width on a
// 32-bit platform and float64 width on a 64-bit platform, from the same
// resolved platform width. The nearest valid range default is therefore
// deterministic per platform, and float literals take the platform float.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 6.3–6.17
//   - rules/types/default_values.md — "Floating and decimal ranges"
func TestPlainFloatFollowsPlatformWidth(t *testing.T) {
	source := `module main

type Ratio float range 0.1..9.0

fn Literal() float {
    return 2.5g
}
`
	for _, test := range []struct {
		width uint16
		bits  int
	}{
		{width: 32, bits: 32},
		{width: 64, bits: 64},
	} {
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: test.width})
		if errors := analyzer.Analyze(program); len(errors) != 0 {
			t.Fatalf("%d-bit errors: %v", test.width, errors)
		}
		if bits := analyzer.types["float"].FloatBits; bits != test.bits {
			t.Fatalf("%d-bit platform float width = %d, want %d", test.width, bits, test.bits)
		}
		resolution := DefaultValueOf(analyzer.types["Ratio"])
		if resolution.Kind != RangeDefault || resolution.Value.Exact == nil {
			t.Fatalf("%d-bit Ratio default = %+v, want a range default", test.width, resolution)
		}
		// The selected value is the nearest representable value at or above
		// 0.1 in the platform width, so the exact values differ by platform.
		want, _ := binaryFloatAtOrAbove(big.NewRat(1, 10), test.bits)
		if resolution.Value.Exact.Cmp(want.Exact) != 0 {
			t.Fatalf("%d-bit Ratio default = %s, want %s", test.width, resolution.Value.Exact.RatString(), want.Exact.RatString())
		}
	}
}
