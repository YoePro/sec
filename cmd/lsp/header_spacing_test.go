package main

import (
	"os"
	"testing"
)

// TestHeaderSpacingLSP uses the shared formatter's canonical header golden.
// Rules: rules/tooling/formatter.md — §28,§29(1–3).
func TestHeaderSpacingLSP(t *testing.T) { assertFormattingGoldenLSP(t, "header_spacing") }

// TestCSTDelimiterSpacingLSP verifies the same parser/CST punctuation policy
// through the user-facing formatting path and its second-pass fixed point.
// Rules: rules/tooling/formatter.md — §28,§29(1–3).
func TestCSTDelimiterSpacingLSP(t *testing.T) { assertFormattingGoldenLSP(t, "cst_delimiter_spacing") }

func assertFormattingGoldenLSP(t *testing.T, fixture string) {
	t.Helper()
	input, err := os.ReadFile("../../testdata/formatter/" + fixture + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/" + fixture + ".expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	if got := formatSource(string(input)); got != string(want) {
		t.Fatalf("LSP output differs:\n%s", got)
	}
	if got := formatSource(string(want)); got != string(want) {
		t.Fatal("LSP formatting is not idempotent")
	}
}

// TestDestructionSyntaxLSP preserves cleanup and consuming markers through
// the same CST formatter used by the command line.
// Rules: rules/tooling/formatter.md — §§21,28,31(8).
func TestDestructionSyntaxLSP(t *testing.T) { assertFormattingGoldenLSP(t, "destruction_syntax") }

// The LSP uses the same declaration-table golden as the CLI.
// Rules: rules/tooling/formatter.md — § 9(1)–(10), § 28(1)–(2), § 29.
func TestBindingAlignmentLSP(t *testing.T) { assertFormattingGoldenLSP(t, "binding_alignment") }
