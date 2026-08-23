package sema

import (
	"math/big"
	"testing"

	"sec/internal/layout"
)

// rules/types/types.md and rules/memory/layout.md; correction5.md requires
// target-sized int and uint bounds to come from the resolved scalar plan.
func TestTargetScalarPlanDefinesIntAndUintBounds(t *testing.T) {
	for _, test := range []struct {
		width   uint16
		intMin  string
		intMax  string
		uintMax string
	}{
		{32, "-2147483648", "2147483647", "4294967295"},
		{64, "-9223372036854775808", "9223372036854775807", "18446744073709551615"},
	} {
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: test.width})
		intType := analyzer.types["int"]
		uintType := analyzer.types["uint"]
		for got, want := range map[string]string{
			intType.MinInteger.String():  test.intMin,
			intType.MaxInteger.String():  test.intMax,
			uintType.MaxInteger.String(): test.uintMax,
		} {
			if got != want {
				t.Fatalf("width %d bound = %s, want %s", test.width, got, want)
			}
		}
		if uintType.MinInteger.Cmp(big.NewInt(0)) != 0 || intType.BitWidth != int64(test.width) || uintType.BitWidth != int64(test.width) {
			t.Fatalf("width %d target types = int:%+v uint:%+v", test.width, intType, uintType)
		}
	}
}
