package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// A statement try with a non-void success value runs natively: success
// continues, failure propagates before the following statements.
// Rules: rules/errors/errorhandling.md — § 8; rules/control-flow/discard.md — §§ 5, 7.
func TestStatementTryValueRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "statement_try")
	if _, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/statement_try_value.sec", "-o", program); code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, program).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42 (another status names the failing case)", err)
	}
}

// The discarded success value of a statement try is an implicit call-result
// discard: a trivial value is dropped, an owned dynamic array destroyed once
// through its plan.
// Rules: rules/control-flow/discard.md — §§ 5, 7; rules/memory/destruction.md — § 12.
func TestStatementTryValueDiscardIR(t *testing.T) {
	for fixture, want := range map[string]string{
		"../../testdata/build/native/statement_try_value.sec": "discard.value %",
		"../../testdata/semantic_ir/statement_try_owned.sec":  `own.discard %4 plan="destroy:array:dynamic::int:int" origin=implicit-call-result`,
	} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", fixture, "-o", "-")
		if code != 0 {
			t.Fatalf("%s: %d %s", fixture, code, stderr)
		}
		if !strings.Contains(stdout, want) || strings.Contains(stdout, "statement try with a non-void") {
			t.Errorf("%s lacks %q:\n%s", fixture, want, stdout)
		}
	}
}
