package main

import (
	"os"
	"strings"
	"testing"
)

// Static pack methods reach the same verified IR/MLIR track as free functions;
// unsupported non-shared cases emit no partial artifact.
// Rules: rules/declarations/functions.md §38A(3),(9); rules/declarations/impl.md §13;
// rules/compiler/semantic_ir.md §27A(2),(4).
func TestTypePackStaticMethodsCLI(t *testing.T) {
	file := typePackCLIFixture(t, "static_methods.sec")
	for _, command := range []string{"emit-ir", "emit-sec-mlir"} {
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
		for _, want := range []string{"Counter.Count[...Types<>]", "Counter.Count[...Types<int,bool>]", "Counter.Count[...Types<bool,int>]", "Other.Count[...Types<int,bool>]", "Counter.Tagged[...Types<int,bool>]"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%s missing %s:\n%s", command, want, stdout)
			}
		}
	}
	for _, name := range []string{"owned_cleanup_boundary.sec"} {
		_, output, code := runCLIForDiagnostics(t, "check", typePackCLIFixture(t, name), "--target", "linux-amd64")
		if code != 0 {
			t.Fatal(name, code, output)
		}
		stdout, output, code := runCLIForDiagnostics(t, "emit-ir", typePackCLIFixture(t, name), "--target", "linux-amd64", "-o", "-")
		if code != 4 || stdout != "" || !strings.Contains(output, "[unsupported-lowering]") {
			t.Fatal(name, code, stdout, output)
		}
	}
}
