package main

import (
	"os"
	"strings"
	"testing"
)

// Completed scalar/result substitutions reach the existing generic IR/MLIR
// pipeline; ambiguous, missing or constrained inference never emits an artifact.
// Rules: rules/declarations/generics.md §§14,18,29; rules/compiler/semantic_ir.md §27A.
func TestTypePackPartialInferenceCLI(t *testing.T) {
	for _, name := range []string{"overloaded_result_context.sec", "overloaded_result_none.sec"} {
		file := typePackCLIFixture(t, name)
		for _, command := range []string{"check", "emit-ir", "emit-sec-mlir"} {
			if command == "emit-sec-mlir" && os.Getenv("SEC_MLIR_BIN") == "" {
				continue
			}
			args := []string{command, file, "--target", "linux-amd64"}
			if command == "emit-ir" {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 0 {
				t.Fatal(name, command, code, stdout, output)
			}
			if command != "check" && !strings.Contains(stdout, "Context[T=int") {
				t.Fatal(name, command, stdout)
			}
		}
	}
	for _, name := range []string{"overloaded_result_ambiguous_invalid.sec", "overloaded_result_missing_invalid.sec", "overloaded_result_constraint_invalid.sec", "overloaded_result_fixed_invalid.sec"} {
		file := typePackCLIFixture(t, name)
		stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
		if code != 3 || stdout != "" || strings.Contains(output, "compiler capability unavailable") {
			t.Fatal(name, code, stdout, output)
		}
	}
}
