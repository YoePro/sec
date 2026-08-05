package ast

import "testing"

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
