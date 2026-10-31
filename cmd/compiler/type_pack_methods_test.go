package main

import (
	"strings"
	"testing"
)

// CLI preserves successful method checking and shared-pack method IR;
// source receiver authority/escape failures remain ordinary semantic errors.
// Rules: rules/declarations/functions.md §§22–24,38A(3–4),(9);
// rules/compiler/semantic_ir.md §27A; rules/memory/borrowing.md §5.4.
func TestTypePackMethodsCLI(t *testing.T) {
	for _, name := range []string{"declarations.sec", "method_calls.sec", "method_reference_return.sec"} {
		file := typePackCLIFixture(t, name)
		_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
		if code != 0 {
			t.Fatal(name, code, output)
		}
	}
	for _, name := range []string{"method_receiver_immutable_invalid.sec", "method_receiver_overlap_invalid.sec", "method_missing_receiver_invalid.sec", "method_receiver_escape_invalid.sec", "method_receiver_move_invalid.sec"} {
		file := typePackCLIFixture(t, name)
		_, output, code := runCLIForDiagnostics(t, "check", file, "--target", "linux-amd64")
		if code != 3 || strings.Contains(output, "compiler capability unavailable") {
			t.Fatal(name, code, output)
		}
	}
	file := typePackCLIFixture(t, "method_ir_boundary.sec")
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", file, "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "Counter.Count[...Types<int>]") {
		t.Fatal(code, stdout, output)
	}
}
