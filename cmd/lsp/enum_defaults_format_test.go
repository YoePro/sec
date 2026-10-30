package main

import "testing"

// TestEnumDefaultFormattingLSP preserves semantic marker placement through the
// shared formatter; it must agree exactly with the CLI golden and fixed point.
// Rules: rules/declarations/enums.md — §8.2;
// rules/tooling/formatter.md — §§15(4),28–29.
func TestEnumDefaultFormattingLSP(t *testing.T) { assertFormattingGoldenLSP(t, "enum_defaults") }
