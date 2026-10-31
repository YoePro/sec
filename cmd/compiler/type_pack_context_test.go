package main

import (
	"os"
	"strings"
	"testing"
)

// Sema fixes must widen the existing track through concrete Semantic IR and,
// when configured, verified Sec MLIR, without using work-in-progress libraries.
// Rules: rules/declarations/functions.md §38A(4),(9); rules/compiler/semantic_ir.md §27A.
func TestTypePackFixedContextCLI(t *testing.T) {
	file := typePackCLIFixture(t, "shared_fixed_context.sec")
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
			t.Fatal(command, code, stdout, output)
		}
		if command != "check" && !strings.Contains(stdout, "Tagged[...Types<int,bool>]") {
			t.Fatal(command, stdout)
		}
	}
}
