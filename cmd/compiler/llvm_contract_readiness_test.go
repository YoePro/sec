package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyLLVMContractReadinessCLI keeps semantic validation ahead of both
// output gates and prevents accepted contracts from losing validation in LLVM.
// Rules: rules/types/contracts.md — Mutation and Conversion failure layers;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func TestLegacyLLVMContractReadinessCLI(t *testing.T) {
	for _, name := range []string{"range", "membership", "multiple", "parity", "finite", "markers", "nested"} {
		t.Run(name, func(t *testing.T) {
			file := "../../testdata/codegen/llvm_contracts/" + name + ".sec"
			_, output, code := runCLIForDiagnostics(t, "sema", file, "--diagnostic-format=json")
			if code != 0 {
				t.Fatal(code, output)
			}
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"emit-llvm", "build"} {
				args := []string{command, file, "-o", path, "--diagnostic-format=json"}
				if command == "build" {
					args = append(args, "--pipeline", "llvm")
				}
				stdout, output, code := runCLIForDiagnostics(t, args...)
				doc := decodeOccurrenceDocument(t, output)
				if code != 4 || stdout != "" || doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
					t.Fatal(command, code, stdout, output)
				}
				failure := doc.Occurrences[0]
				if failure.Arguments["category"] != "unsupported-lowering" || !strings.Contains(failure.Message.Text, "contract validation") || failure.Primary == nil || len(failure.Help) == 0 {
					t.Fatal(failure)
				}
				contents, err := os.ReadFile(path)
				if err != nil || string(contents) != "existing output" {
					t.Fatal(string(contents), err)
				}
			}
		})
	}
	_, output, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/codegen/llvm_contracts/value_invalid.sec", "-o", "-", "--diagnostic-format=json")
	if code != 3 || !strings.Contains(output, "S1068") {
		t.Fatal(code, output)
	}
	stdout, output, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/codegen/llvm_contracts/plain.sec", "-o", "-", "--diagnostic-format=json")
	if code != 0 || !strings.Contains(stdout, "define i32 @Identity") {
		t.Fatal(code, stdout, output)
	}
}

// Rules: rules/types/contracts.md — String and collection contracts.
// Complete analyzed string length conjunctions pass the native LLVM output gate;
// raw generator entry points still require exact Sema facts.
func TestLegacyLLVMStringLengthReadinessCLI(t *testing.T) {
	stdout, output, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/codegen/llvm_contracts/length.sec", "-o", "-", "--diagnostic-format=json")
	if code != 0 || !strings.Contains(stdout, "define i64 @main") {
		t.Fatal(code, stdout, output)
	}
}
