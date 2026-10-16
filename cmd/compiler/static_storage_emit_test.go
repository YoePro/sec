package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emit-ir prints static storage as module statics with compile-time
// initializers, and emit-sec-mlir rejects it — leaving existing output
// untouched — because no Sec MLIR schema represents static storage yet.
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
	path := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "-o", path)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "static storage") {
		t.Fatal(code, stdout, stderr)
	}
	if contents, err := os.ReadFile(path); err != nil || string(contents) != "existing output" {
		t.Fatal(string(contents), err)
	}
}

// The legacy direct emit-llvm and emit-mlir routes have no static storage:
// they reject a function-local static instead of lowering it to a stack slot
// that is re-initialized on every call (which made Next() always return 1).
//
// Rules:
//   - rules/declarations/static.md — § 4
func TestLegacyRoutesRejectFunctionLocalStatics(t *testing.T) {
	file := "../../testdata/semantic_ir/static_local_valid.sec"
	for _, command := range []string{"emit-llvm", "emit-mlir"} {
		stdout, stderr, code := runCLIForDiagnostics(t, command, file, "-o", "-")
		if code == 0 || strings.Contains(stdout, "alloca") || !strings.Contains(stderr, "function-local static storage") {
			t.Fatal(command, code, stdout, stderr)
		}
	}
}
