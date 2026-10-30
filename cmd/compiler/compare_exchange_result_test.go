package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// TestCompareExchangeResultCLI loads the real core declaration and checks its
// nominal payload, default, match and non-error surface through the compiler.
// Rules: rules/concurrency/atomics.md — §§4,6,17,43–45.
func TestCompareExchangeResultCLI(t *testing.T) {
	const root = "../../testdata/sema/atomics/compare_exchange_result/"
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"compare_exchange_result_valid.sec", 0, 0},
		{"lowering_valid.sec", 0, 0},
		{"compare_exchange_result_invalid.sec", 3, 13},
		{"compare_exchange_result_spoof_invalid.sec", 3, 1},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", root+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(test.file, code, stderr)
		}
		if test.errors != 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
	}
}

// TestCompareExchangeResultSemanticIR preserves the associated value through
// the existing verified sum construction and match path, and the native track
// returns it from the executed match (lowering v11.33).
// Rules: rules/concurrency/atomics.md — §6; rules/compiler/semantic_ir.md — Sum types;
// rules/compiler/compiler_pipeline.md — Unsupported lowering.
func TestCompareExchangeResultSemanticIR(t *testing.T) {
	const file = "../../testdata/sema/atomics/compare_exchange_result/lowering_valid.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, text := range []string{"core::CompareExchangeResult<int>", "#0 Exchanged empty", "#1 NotExchanged single", "union.construct variant=#1", "union.unwrap-payload"} {
		if !strings.Contains(stdout, text) {
			t.Fatal(text, stdout)
		}
	}
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "result")
	_, stderr, code = runCLIForDiagnostics(t, "build", file, "-o", program)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	var exit *exec.ExitError
	if err := exec.Command(program).Run(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("status = %v, want the observed value 7", err)
	}
}
