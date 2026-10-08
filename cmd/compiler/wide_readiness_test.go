package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyWideReadinessCLI covers every selected legacy pipeline, semantic
// acceptance before capability rejection, diagnostics and output preservation.
// Rules: rules/types/types.md — active wide types;
// rules/foundations/operators.md — Checked integer arithmetic;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func TestLegacyWideReadinessCLI(t *testing.T) {
	data, err := os.ReadFile("../../testdata/codegen/wide_legacy/operation.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"int128", "uint128", "int256", "uint256", "decimal128"} {
		t.Run(typ, func(t *testing.T) {
			source := strings.ReplaceAll(strings.ReplaceAll(string(data), "TYPE", typ), "OP", "+")
			file := filepath.Join(t.TempDir(), "input.sec")
			if err := os.WriteFile(file, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, code := runCLIForDiagnostics(t, "sema", file, "--diagnostic-format=json")
			if code != 0 {
				t.Fatal(code, diagnostics)
			}
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"emit-llvm"}, {"emit-mlir"}, {"build", "--pipeline", "llvm"}, {"build", "--pipeline", "mlir"}} {
				args = append(args, file, "-o", path, "--diagnostic-format=json")
				stdout, diagnostics, code := runCLIForDiagnostics(t, args...)
				doc := decodeOccurrenceDocument(t, diagnostics)
				if code != 4 || stdout != "" || len(doc.Occurrences) != 1 {
					t.Fatal(args, code, stdout, diagnostics)
				}
				failure := doc.Occurrences[0]
				if failure.Arguments["category"] != "unsupported-lowering" || failure.Primary == nil || len(failure.Help) == 0 || !strings.Contains(failure.Message.Text, "wide") && !strings.Contains(failure.Message.Text, "decimal128") {
					t.Fatal(failure)
				}
				contents, err := os.ReadFile(path)
				if err != nil || string(contents) != "existing output" {
					t.Fatal(string(contents), err)
				}
			}
		})
	}
}
