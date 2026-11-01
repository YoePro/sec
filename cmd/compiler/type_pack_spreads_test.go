package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// Forwarded owned packs run natively with their position order; forwarded
// reference packs reach verified Sec MLIR and wait for P24 to execute.
// Rules: rules/declarations/functions.md §38A(7); rules/compiler/semantic_ir.md §27A(7).
func TestTypePackSpreadsNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "pack-spread")
	if _, output, code := runCLIForDiagnostics(t, "build", typePackCLIFixture(t, "spread_forward_owned.sec"), "--target", "linux-amd64", "-o", program); code != 0 {
		t.Fatal(code, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, program).CombinedOutput(); err != nil {
		t.Fatalf("forwarding program: %v %s", err, output)
	}
	stdout, output, code := runCLIForDiagnostics(t, "emit-sec-mlir", typePackCLIFixture(t, "spread_forward_references.sec"), "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "reborrow") {
		t.Fatal(code, output)
	}
}

// Pack methods of generic owners reach verified Sec MLIR through the single track.
func TestTypePackGenericOwnerMethodsMLIR(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	stdout, output, code := runCLIForDiagnostics(t, "emit-sec-mlir", typePackCLIFixture(t, "method_generic_owner.sec"), "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "main::Box<bool>") || !strings.Contains(stdout, "main::Box<int>") {
		t.Fatal(code, output)
	}
}

// Forwarded overload selection, static methods and trailing arguments run with
// their concrete callee; instance method forwards reach verified Sec MLIR.
func TestTypePackForwardTargets(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "pack-targets")
	if _, output, code := runCLIForDiagnostics(t, "build", typePackCLIFixture(t, "spread_forward_overloads.sec"), "--target", "linux-amd64", "-o", program); code != 0 {
		t.Fatal(code, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, program).CombinedOutput(); err != nil {
		t.Fatalf("forward target program: %v %s", err, output)
	}
	if _, output, code := runCLIForDiagnostics(t, "emit-sec-mlir", typePackCLIFixture(t, "spread_forward_targets.sec"), "--target", "linux-amd64", "-o", "-"); code != 0 {
		t.Fatal(code, output)
	}
}

// Returned references through forwards reach verified Sec MLIR; they run after P24.
func TestTypePackForwardReturnsMLIR(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, output, code := runCLIForDiagnostics(t, "emit-sec-mlir", typePackCLIFixture(t, "spread_forward_returns.sec"), "--target", "linux-amd64", "-o", "-"); code != 0 {
		t.Fatal(code, output)
	}
}
