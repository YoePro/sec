package main

import (
	"errors"
	"os/exec"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/testsupport"
)

// Only impossible argument checks disappear; unproven failures remain channel
// errors and cannot emit an artifact.
// Rules: rules/errors/errorhandling.md — §§11,13.1.
func TestTryArgumentArithmeticProofCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/try_arithmetic_proof/valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/try_arithmetic_proof/unproven_invalid.sec", "--diagnostic-format=json", "-o", "-")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || stdout != "" || len(doc.Occurrences) != 3 {
		t.Fatal(code, stdout, stderr)
	}
	for _, item := range doc.Occurrences {
		if item.ID == nil || *item.ID != diagnostics.TryPropagationIncompatible {
			t.Fatal(item)
		}
	}
}

// Checked native arithmetic remains in the track; widened int32 extrema pass
// through a BusinessError-returning call without inventing ArithmeticError.
// Rules: rules/errors/errorhandling.md — §§11,13.1;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestTryArgumentArithmeticProofNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	var exit *exec.ExitError
	if err := runNativeProgram(t, "../../testdata/build/native/try_safe_argument.sec"); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatal(err)
	}
}
