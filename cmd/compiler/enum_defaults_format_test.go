package main

import "testing"

// TestEnumDefaultFormattingCLI uses the same canonical default-member golden
// as the shared formatter and editor, including its second-pass fixed point.
// Rules: rules/declarations/enums.md — §8.2;
// rules/tooling/formatter.md — §§15(4),28–29.
func TestEnumDefaultFormattingCLI(t *testing.T) { assertFormattingGoldenCLI(t, "enum_defaults") }
