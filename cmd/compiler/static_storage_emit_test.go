package main

import (
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// emit-ir prints static storage as module statics with compile-time
// initializers; emit-sec-mlir publishes them as sec.static.global with
// symbol loads and stores, and the compilation track lowers them to LLVM
// globals, constant when immutable (user decision 2026-10-18).
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 28
//   - rules/compiler/compiler_pipeline.md — §§ 32–34
func TestStaticStorageEmitRoutes(t *testing.T) {
	file := "../../testdata/semantic_ir/static_storage_valid.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file, "-o", "-")
	if code != 0 || !strings.Contains(stdout, `"main::Next::calls"`) || !strings.Contains(stdout, "static.load @") {
		t.Fatal(code, stdout, stderr)
	}
	testsupport.RequireSecMLIROptPath(t)
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "-o", "-")
	if code != 0 || !strings.Contains(stdout, `"sec.static.global"`) || !strings.Contains(stdout, `"sec.static.load"`) {
		t.Fatal(code, stdout, stderr)
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-llvm", file, "-o", "-")
	if code != 0 || !strings.Contains(stdout, "internal constant i64 7") || !strings.Contains(stdout, "internal global i64 0") {
		t.Fatal(code, stdout, stderr)
	}
}
