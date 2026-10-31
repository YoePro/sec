package main

import (
	"os"
	"strings"
	"testing"
)

// Instance pack calls preserve receiver authority through verified Semantic IR
// and Sec MLIR; invalid receiver lifetimes remain source errors before emission.
// Rules: rules/declarations/functions.md §§22,38A(8–9);
// rules/compiler/semantic_ir.md §§27A,60(3),61(4).
func TestTypePackInstanceMethodsCLI(t *testing.T) {
	for _, name := range []string{"instance_methods.sec", "method_reference_return.sec"} {
		for _, command := range []string{"emit-ir", "emit-sec-mlir"} {
			if command == "emit-sec-mlir" && os.Getenv("SEC_MLIR_BIN") == "" {
				continue
			}
			args := []string{command, typePackCLIFixture(t, name), "--target", "linux-amd64"}
			if command == "emit-ir" {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 0 {
				t.Fatal(name, command, code, stdout, output)
			}
			if !strings.Contains(stdout, "Counter.") || !strings.Contains(stdout, "[...Types<") {
				t.Fatal(name, command, stdout)
			}
		}
	}
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", typePackCLIFixture(t, "method_receiver_escape_invalid.sec"), "--target", "linux-amd64", "-o", "-")
	if code != 3 || stdout != "" || strings.Contains(output, "[unsupported-lowering]") {
		t.Fatal(code, stdout, output)
	}
}
