package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Frontend API support never substitutes for the P23 materializer: supported
// source checks, but absent allocation/string lowering must leave no artifact.
// Rules: rules/corrections/md109-core-string-materialization-correction.md
// — §§5.2,10.3,12.1–12.6; rules/mlir/packages/sec-mlir-dialect_package23.md — §§35–37.
func TestStringMaterializationLoweringBoundary(t *testing.T) {
	for _, file := range []string{"string_materialization_bytes.sec", "string_materialization_runes.sec"} {
		t.Run(file, func(t *testing.T) {
			input := "../../testdata/semantic_ir/" + file
			if _, stderr, code := runCLIForDiagnostics(t, "check", input); code != 0 {
				t.Fatalf("public source API rejected: %d %s", code, stderr)
			}
			output := filepath.Join(t.TempDir(), "materialized.mlir")
			stdout, stderr, code := runCLIForDiagnostics(t, "emit-mlir", input, "-o", output)
			if code == 0 || stdout != "" || !strings.Contains(stderr, "unsupported-lowering") || !strings.Contains(stderr, "materialization requires P23 allocation/string lowering") {
				t.Fatalf("missing P23 capability was not rejected: %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("unsupported materialization left an artifact: %v", err)
			}
			t.Logf("current P23 boundary: %s", stderr)
		})
	}
}
