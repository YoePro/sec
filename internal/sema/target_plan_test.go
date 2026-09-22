package sema

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
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

// Explicit constant conversions must use the target-selected int and uint
// representation, rather than the host width or a fixed frontend assumption.
//
// Rules:
//   - rules/types/types.md — "Explicit conversions"
//   - rules/types/types.md — "int and uint"
func TestTargetScalarPlanChecksExplicitIntegerConstantConversions(t *testing.T) {
	source := `module main

fn Values() void {
	let signedMaximum := int(2147483647)
	let signedMinimum := int(-2147483648)
	let unsignedMaximum := uint(4294967295u)
	let signedOverflow := int(2147483648)
	let signedUnderflow := int(-2147483649)
	let unsignedOverflow := uint(4294967296u)
}
`

	program := parser.New(lexer.New(source)).ParseProgram()
	for _, test := range []struct {
		width      uint16
		wantErrors []string
	}{
		{32, []string{
			"value 2147483648 overflows int",
			"value -2147483649 overflows int",
			"value 4294967296 overflows uint",
		}},
		{64, nil},
	} {
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: test.width})
		errors := analyzer.Analyze(program)
		if len(errors) != len(test.wantErrors) {
			t.Fatalf("width %d errors = %v, want %v", test.width, errors, test.wantErrors)
		}
		for i, want := range test.wantErrors {
			if !strings.Contains(errors[i].Message, want) {
				t.Errorf("width %d error %d = %q, want substring %q", test.width, i, errors[i].Message, want)
			}
		}
	}
}
