package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Safe references have no physical representation or ABI yet: every emit
// route rejects a valid program that uses them, prints nothing and leaves an
// existing output file untouched (P15-67: physical reference ABI/layout and
// LLVM lowering are outside Package 15).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 3, 155
//   - rules/compiler/compiler_pipeline.md — §§ 32–34
func TestReferenceProgramsAreRejectedByEveryEmitRoute(t *testing.T) {
	file := "../../testdata/semantic_ir/reference_emit_boundary.sec"
	if _, stderr, code := runCLIForDiagnostics(t, "check", file); code != 0 {
		t.Fatal(code, stderr)
	}
	for _, command := range []string{"emit-ir", "emit-sec-mlir", "emit-llvm", "emit-mlir"} {
		t.Run(command, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCLIForDiagnostics(t, command, file, "-o", path)
			if code == 0 || stdout != "" || stderr == "" {
				t.Fatal(code, stdout, stderr)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != "existing output" {
				t.Fatal(string(contents), err)
			}
		})
	}
}
