package main

import (
	"strings"
	"testing"
)

// rules/memory/unsafe.md §7(1–5): all frontend gates must reject implicit
// unsafe permission in a callable body, before producing backend output.
func TestUnsafeFunctionBodyDiagnosticsCLI(t *testing.T) {
	for _, fixture := range []string{"raw_pointer_unsafe_body_invalid.sec", "unsafe_function_asm_invalid.sec"} {
		for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
			args := []string{command, "../../testdata/tooling/" + fixture, "--diagnostic-format=json"}
			if command != "sema" && command != "analyse" {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			doc := decodeOccurrenceDocument(t, output)
			if code != 3 || stdout != "" || doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
				t.Fatalf("%s: code=%d stdout=%q diagnostics=%s", command, code, stdout, output)
			}
		}
	}
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/tooling/unsafe_callable_body_valid.sec")
	if code != 0 {
		t.Fatalf("explicit contexts: code=%d %s", code, output)
	}
}

// The named output is returned inside its explicit block and remains available
// to LLVM emission without extending the asm output's lexical scope.
// Rules: rules/memory/unsafe.md — §7(1–3);
// rules/platform/inline_assembly.md — §30.
func TestUnsafeNamedAsmOutputCLI(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/codegen/unsafe_named_asm_output.sec", "--target", "linux-amd64", "-o", "-")
	if code != 0 || !strings.Contains(stdout, "call i64 asm sideeffect") || !strings.Contains(stdout, "ret i64") {
		t.Fatal(code, stdout, stderr)
	}
}
