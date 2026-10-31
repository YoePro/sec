package main

import (
	"os"
	"strings"
	"testing"
)

// Argument-selected overloads must flow through the existing specialization
// pipeline; ambiguity remains a semantic error, not a blanket capability gate.
// Rules: rules/declarations/functions.md §38A(4)(5),(9); rules/compiler/semantic_ir.md §27A.
func TestTypePackOverloadsCLI(t *testing.T) {
	file := typePackCLIFixture(t, "overloaded_calls.sec")
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
		if command != "check" && (!strings.Contains(stdout, "Pick[...Types<bool>]") || !strings.Contains(stdout, "Pick[...Types<int>]")) {
			t.Fatal(command, stdout)
		}
	}
	for _, name := range []string{"shared_return_overload_invalid.sec", "overloaded_owning_ambiguous_invalid.sec"} {
		file := typePackCLIFixture(t, name)
		stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
		if code != 3 || stdout != "" || !strings.Contains(output, "ambiguous call") || strings.Contains(output, "compiler capability unavailable") {
			t.Fatal(name, code, stdout, output)
		}
	}
}
