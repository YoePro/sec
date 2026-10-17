package main

import (
	"strings"
	"testing"
)

// `atomic.Fence(order)` resolves in the core atomic namespace, requires one
// explicit compile-time MemoryOrder (no default), accepts Acquire, Release,
// AcqRel and SeqCst and rejects Relaxed with the canonical diagnostic; a
// local value named `atomic` keeps ordinary method lookup.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(3), 24, 25(5), 26, 45(9), 48
func TestAtomicFenceSurface(t *testing.T) {
	for _, file := range []string{"atomic_fence_valid.sec", "atomic_fence_shadowed_valid.sec"} {
		if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+file); code != 0 {
			t.Fatalf("%s: %d %s", file, code, stderr)
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/atomic_fence_invalid.sec")
	if code == 0 {
		t.Fatal("invalid fences accepted")
	}
	for _, want := range []string{
		"S1171]: MemoryOrder.Relaxed is not valid for atomic.Fence; allowed orders: Acquire, Release, AcqRel, SeqCst",
		"S1171]: atomic.Fence requires exactly one MemoryOrder argument, got 0",
		"S1170]: atomic.Fence order argument order cannot be evaluated at compile time",
		"S1171]: atomic.Fence order must be MemoryOrder, got int",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "undefined variable atomic") || strings.Contains(stderr, "unknown function or type atomic.Fence") {
		t.Errorf("atomic.Fence was not resolved:\n%s", stderr)
	}
}

// emit-ir prints the atomic.fence operation with its established order;
// Sec MLIR schema 11 has no fence operation and rejects the module with an
// explicit unsupported-lowering error and no output (atomics.md § 40(2)).
func TestAtomicFenceEmitRoutes(t *testing.T) {
	file := "../../testdata/sema/atomic_fence_lowering_valid.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 || !strings.Contains(stdout, "atomic.fence order=Release") {
		t.Fatalf("emit-ir: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "does not support atomic.fence") {
		t.Fatalf("emit-sec-mlir: %d %q %s", code, stdout, stderr)
	}
}
