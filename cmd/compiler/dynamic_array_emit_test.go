package main

import (
	"os"
	"strings"
	"testing"
)

// The inspection routes reach owning dynamic arrays: emit-ir prints the P18
// Semantic IR and emit-sec-mlir publishes verified schema-14 Sec MLIR on 64-
// and 32-bit targets, with the dynamic-array protocol verifier in the CLI
// verification pipeline.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 81, 128–131
//   - rules/compiler/compiler_pipeline.md — § 59(4)–(7) (MD-086)
func TestDynamicArraysReachTheInspectionBoundaries(t *testing.T) {
	file := "../../testdata/semantic_ir/dynamic_array_schema14.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", file)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, want := range []string{"dynamic-array.empty", "dynamic-array.grow-backing", "dynamic-array.append-prepared", "dynamic-array.element-place"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("emit-ir lacks %q", want)
		}
	}
	bin := os.Getenv("SEC_MLIR_BIN")
	if bin == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file, "--target", target, "--mlir-bin", bin)
		if code != 0 {
			t.Fatal(target, code, stderr)
		}
		for _, want := range []string{"sec.dialect_version = 14 : i32", `"sec.dynamic_array.append_prepared"(`, `"sec.storage.establish_domain"(`, `kind = "dynamic-array"`} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%s emit-sec-mlir lacks %q", target, want)
			}
		}
	}
}
