package main

import "testing"

// CLI stdin formatting shares general attribute layout and stable output.
// Rules: rules/tooling/formatter.md §§16(14–17),28–29.
func TestGeneralAttributeFormattingCLI(t *testing.T) {
	assertFormattingGoldenCLI(t, "attributes/general")
}
