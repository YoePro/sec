package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyLLVMFloatReadinessDiagnostics verifies frontend-valid programs are
// rejected only at legacy LLVM lowering, with source context and no output write.
// Rules: rules/types/types.md — "Binary floating-point types"; MD-014 §6;
// rules/tooling/diagnostics.md — "Shared model".
func TestLegacyLLVMFloatReadinessDiagnostics(t *testing.T) {
	for _, name := range []string{"signature", "conversion", "alias", "field", "nested", "default", "inferred_fraction", "inferred_integer"} {
		t.Run(name, func(t *testing.T) {
			file := "../../testdata/codegen/llvm_platform_float/" + name + ".sec"
			_, output, code := runCLIForDiagnostics(t, "sema", file, "--diagnostic-format=json")
			if code != 0 {
				t.Fatal(code, output)
			}
			path := filepath.Join(t.TempDir(), "result.ll")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, output, code := runCLIForDiagnostics(t, "emit-llvm", file, "-o", path, "--diagnostic-format=json")
			doc := decodeOccurrenceDocument(t, output)
			if code != 4 || stdout != "" || doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
				t.Fatal(code, stdout, output)
			}
			failure := doc.Occurrences[0]
			if failure.Arguments["category"] != "unsupported-lowering" || !strings.Contains(failure.Message.Text, "float") || failure.Primary == nil || failure.Primary.Span.Start.Line == 0 || len(failure.Help) == 0 {
				t.Fatal(failure)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != "existing output" {
				t.Fatal(string(contents), err)
			}
		})
	}
}

// TestLegacyLLVMBuildRejectsPlatformFloat stops before invoking LLVM tools or
// replacing a binary when the backend lacks the selected float representation.
// Rules: rules/types/types.md — "Binary floating-point types"; MD-014 §6.
func TestLegacyLLVMBuildRejectsPlatformFloat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program")
	if err := os.WriteFile(path, []byte("existing binary"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, output, code := runCLIForDiagnostics(t, "build", "../../testdata/codegen/llvm_platform_float/signature.sec", "--pipeline", "llvm", "-o", path, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 4 || stdout != "" || doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 || doc.Occurrences[0].Arguments["category"] != "unsupported-lowering" {
		t.Fatal(code, stdout, output)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "existing binary" {
		t.Fatal(string(contents), err)
	}
}
