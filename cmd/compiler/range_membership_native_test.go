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

const rangeMembershipFixture = "../../testdata/build/native/range_membership_scalars.sec"

// Range membership over char and rune lowers to unsigned integer comparisons
// and over binary floats to the ordered float comparisons, so a NaN value or
// bound is never contained.
// Rules: rules/foundations/operators.md — §§ 63, 65; rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 24–26.
func TestRangeMembershipScalarsIR(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", rangeMembershipFixture, "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	for _, want := range []string{"int.compare ge", "int.compare lt", "float.compare ge", "predicate=oge", "predicate=ole", "predicate=olt"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("emit-ir lacks %q", want)
		}
	}
	if strings.Contains(stdout, "range membership over") {
		t.Error("range membership still reports a boundary")
	}
}

// The fixture runs on the native track and returns 42 only when every
// boundary, exclusive end, open range, negation, NaN and infinity case holds.
// Rules: rules/foundations/operators.md — § 65.
func TestRangeMembershipScalarsRun(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "range")
	if _, stderr, code := runCLIForDiagnostics(t, "build", rangeMembershipFixture, "-o", program); code != 0 {
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
