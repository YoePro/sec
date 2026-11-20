package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// TestUnionPreparationPipeline carries saved operand order, default values,
// generic types and payload actions through the single compilation track.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/compiler/semantic_ir.md §§23,35–36.
func TestUnionPreparationPipeline(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	fixture := "../../testdata/semantic_ir/union_preparation_consumers.sec"
	ir, stderr, code := runCLIForDiagnostics(t, "emit-ir", fixture, "-o", "-")
	if code != 0 || !strings.Contains(ir, "union.construct") {
		t.Fatalf("IR: %d %s %s", code, stderr, ir)
	}
	mlir, stderr, code := runCLIForDiagnostics(t, "emit-mlir", fixture, "-o", "-")
	if code != 0 {
		t.Fatalf("MLIR: %d %s", code, stderr)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = strings.NewReader(mlir)
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verify: %v %s", err, output)
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "union-preparation")
	if _, stderr, code := runCLIForDiagnostics(t, "build", fixture, "-o", program); code != 0 {
		t.Fatalf("native: %d %s", code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, program).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("native result: %v, want 42", err)
	}
}
