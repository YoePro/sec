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

// `x op= try e` on locals and mutable statics runs on the native track: the
// destination is read first, a failing source leaves through the try or its
// handler, and the destination never changes on failure.
// Rules: rules/foundations/operators.md — §§ 84(3), 87(3).
func TestCompoundAssignmentTryRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "compound")
	if _, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/compound_assignment_try.sec", "-o", program); code != 0 {
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

// Through a mutable reference the referent read precedes the try source, the
// failure edge returns before the write, and the write follows the checked
// operator.
// Rules: rules/foundations/operators.md — §§ 84(3), 87(3).
func TestCompoundAssignmentTryThroughReferenceIR(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/build/native/compound_assignment_try_reference.sec", "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	start := strings.Index(stdout, `func "main::AddThrough`)
	if start < 0 {
		t.Fatalf("missing AddThrough:\n%s", stdout)
	}
	body := stdout[start:]
	if end := strings.Index(body[1:], "\n  func "); end >= 0 {
		body = body[:end+1]
	}
	order := []string{"place.read", `call.direct "main::Parse(int)"`, "result.is-err", "result.unwrap-ok", "int.binary-checked add", "place.write"}
	position := -1
	for _, want := range order {
		next := strings.Index(body, want)
		if next <= position {
			t.Fatalf("%q is missing or out of order:\n%s", want, body)
		}
		position = next
	}
}
