package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// TestHeaderSpacingCLI verifies the actual stdin command agrees with the shared
// golden and LSP, including its second-pass fixed point and diagnostics stream.
// Rules: rules/tooling/formatter.md — §28,§29(1–3).
func TestHeaderSpacingCLI(t *testing.T) { assertFormattingGoldenCLI(t, "header_spacing") }

// TestCSTDelimiterSpacingCLI verifies the same parser/CST punctuation policy
// through the user-facing formatting path and its second-pass fixed point.
// Rules: rules/tooling/formatter.md — §28,§29(1–3).
func TestCSTDelimiterSpacingCLI(t *testing.T) { assertFormattingGoldenCLI(t, "cst_delimiter_spacing") }

func assertFormattingGoldenCLI(t *testing.T, fixture string) {
	t.Helper()
	input, err := os.ReadFile("../../testdata/formatter/" + fixture + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/" + fixture + ".expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range [][]byte{input, want} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFmtCheckCLIProcess$", "--", "fmt", "--stdin")
		cmd.Env = append(os.Environ(), "SEC_FMT_CHECK_TEST_PROCESS=1")
		cmd.Stdin = bytes.NewReader(source)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil || !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 {
			t.Fatalf("CLI formatting: %v\n%s\n%s", err, stdout.String(), stderr.String())
		}
	}
}

// TestDestructionSyntaxCLI preserves explicit cleanup and ownership markers.
// Rules: rules/tooling/formatter.md — §§21,28,31(8).
func TestDestructionSyntaxCLI(t *testing.T) { assertFormattingGoldenCLI(t, "destruction_syntax") }
