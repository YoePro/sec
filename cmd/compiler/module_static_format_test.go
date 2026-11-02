package main

import "testing"

// CLI formatting removes redundant module static and preserves required members.
// Rules: rules/declarations/static.md — §§3,25; rules/tooling/formatter.md — §§28–29.
func TestModuleStaticFormattingCLI(t *testing.T) { assertFormattingGoldenCLI(t, "module_static") }
