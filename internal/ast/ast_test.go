package ast

import (
	"math"
	"testing"
)

func TestNumericLiteralParsersIgnoreDigitSeparators(t *testing.T) {
	integer, ok := ParseIntegerLiteralLexeme("0xFFFF_FFFF")
	if !ok || integer.String() != "4294967295" {
		t.Fatalf("hex value = %v, %t", integer, ok)
	}

	floating, ok := ParseFloatLiteralFloat64("1_234.5_678")
	if !ok || floating != 1234.5678 {
		t.Fatalf("float value = %v, %t", floating, ok)
	}

	if got := NormalizeNumericLiteralLexeme("1.25e1_000"); got != "1.25e1000" {
		t.Fatalf("normalized lexeme = %q", got)
	}
}

func TestIntegerFormFamilySuffixesPreserveBasePrefixedValue(t *testing.T) {
	integer, ok := ParseIntegerFormNumericLiteralLexeme("0x41t")
	if !ok || integer.String() != "65" {
		t.Fatalf("char-shaped hex value = %v, %t", integer, ok)
	}

	floating, ok := ParseFloatLiteralFloat64("0x10g")
	if !ok || floating != 16 {
		t.Fatalf("float-shaped hex value = %v, %t", floating, ok)
	}

	integer, ok = ParseIntegerFormNumericLiteralLexeme("0x10m")
	if !ok || integer.String() != "16" {
		t.Fatalf("decimal-shaped hex value = %v, %t", integer, ok)
	}
}

// Approximate AST float64 presentation must not reject well-formed exact
// decimal sources, but range tolerance must not swallow malformed syntax.
// Rules: rules/foundations/lexical_structure.md — §12.9;
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §3(7).
func TestNumericLiteralHostRangeDoesNotLimitParsing(t *testing.T) {
	for _, text := range []string{"1e400", "1e400m", "1e400g"} {
		value, ok := ParseFloatLiteralFloat64(text)
		if !ok || !math.IsInf(value, 1) {
			t.Errorf("%s: %v %t", text, value, ok)
		}
	}
	for _, text := range []string{"1e-400", "1e-400m", "1e-400g"} {
		value, ok := ParseFloatLiteralFloat64(text)
		if !ok || value != 0 {
			t.Errorf("%s: %v %t", text, value, ok)
		}
	}
	for _, text := range []string{"1e", "1e+oops", "1.2.3"} {
		if _, ok := ParseFloatLiteralFloat64(text); ok {
			t.Errorf("malformed %s accepted", text)
		}
	}
}
