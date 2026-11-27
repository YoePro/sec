package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// TestCheckedUnreachableReachesPanicEndpoint: a reachable checked
// `unreachable` lowers through the registered UnreachableReached panic
// (Semantic IR), sec.fail.panic (Sec MLIR) and the defined panic endpoint
// llvm.trap before the backend `unreachable` (LLVM IR). A statement Sema
// proves unreachable is rejected as S3001 rather than lowered.
// Rules: rules/errors/panic.md — §§ 13(1), 15–16; rules/errors/runtime_checks.md — § 39.
func TestCheckedUnreachableReachesPanicEndpoint(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	file := "../../testdata/sema/unreachable_panic_endpoint.sec"
	ir, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 || strings.Count(ir, "panic reason=UnreachableReached") != 1 {
		t.Fatalf("emit-ir must keep exactly the reachable checked unreachable (%d):\n%s\n%s", code, ir, stderr)
	}
	mlir, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file)
	if code != 0 || strings.Count(mlir, `"sec.fail.panic"() <{reason = "UnreachableReached"}>`) != 1 {
		t.Fatalf("emit-sec-mlir (%d):\n%s\n%s", code, mlir, stderr)
	}
	path := filepath.Join(t.TempDir(), "out.ll")
	if _, stderr, code := runCLIForDiagnostics(t, "emit-llvm", file, "-o", path); code != 0 {
		t.Fatalf("emit-llvm %d: %s", code, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	llvm := string(data)
	trap := strings.Index(llvm, "call void @llvm.trap()")
	if trap < 0 || !strings.Contains(llvm[trap:], "unreachable") || strings.Count(llvm, "call void @llvm.trap()") != 1 {
		t.Fatalf("the reachable unreachable must reach one llvm.trap before the backend unreachable:\n%s", llvm)
	}
}

// TestAssertionFailureReachesPanicEndpoint: an unproven assertion lowers to
// its check and the registered AssertionFailed panic with its static message
// (Semantic IR, Sec MLIR) and to the selected non-returning endpoint
// llvm.trap in LLVM IR, with no dynamic allocation on the failure path.
// Rules: rules/errors/panic.md — §§ 13(1), 14, 15; rules/errors/runtime_checks.md — § 39.
func TestAssertionFailureReachesPanicEndpoint(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	file := "../../testdata/sema/assert_panic_endpoint.sec"
	ir, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 || !strings.Contains(ir, `panic reason=AssertionFailed#6 message="count must be positive"`) {
		t.Fatalf("emit-ir (%d):\n%s\n%s", code, ir, stderr)
	}
	mlir, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file)
	if code != 0 || !strings.Contains(mlir, `"sec.fail.panic"() <{message = "count must be positive", reason = "AssertionFailed"}>`) {
		t.Fatalf("emit-sec-mlir (%d):\n%s\n%s", code, mlir, stderr)
	}
	path := filepath.Join(t.TempDir(), "out.ll")
	if _, stderr, code := runCLIForDiagnostics(t, "emit-llvm", file, "-o", path); code != 0 {
		t.Fatalf("emit-llvm %d: %s", code, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	llvm := string(data)
	trap := strings.Index(llvm, "call void @llvm.trap()")
	if trap < 0 || !strings.Contains(llvm[trap:], "unreachable") {
		t.Fatalf("assertion failure must reach the trap endpoint:\n%s", llvm)
	}
	for _, allocation := range []string{"@malloc", "@calloc", "@realloc", "@sec_alloc"} {
		if strings.Contains(llvm, allocation) {
			t.Fatalf("the panic path must not allocate (%s):\n%s", allocation, llvm)
		}
	}
}

