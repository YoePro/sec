package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAllocationLoweringSelectionCLI distinguishes valid source from missing
// allocator routing and preserves an existing output file on every emit route.
// Rules: rules/memory/allocation.md — §§25(5),26(1)–(3),28(4);
// rules/compiler/compiler_pipeline.md — §§32–34,58(3).
func TestAllocationLoweringSelectionCLI(t *testing.T) {
	for _, name := range []string{"domain", "backing", "context", "growth"} {
		file := "../../testdata/sema/allocation_lowering_" + name + "_valid.sec"
		if _, stderr, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json"); code != 0 {
			t.Fatal(name, code, stderr)
		}
		for _, command := range []string{"emit-ir", "emit-sec-mlir", "emit-llvm", "emit-mlir"} {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCLIForDiagnostics(t, command, file, "-o", path, "--diagnostic-format=json")
			doc := decodeOccurrenceDocument(t, stderr)
			if code != 4 || stdout != "" || len(doc.Occurrences) != 1 {
				t.Fatal(command, name, code, stdout, stderr)
			}
			failure := doc.Occurrences[0]
			if failure.Arguments["category"] != "unsupported-lowering" || !strings.Contains(failure.Message.Text, "allocation") || failure.Primary == nil || failure.Primary.Span.Start.Line == 0 {
				t.Fatal(failure)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != "existing output" {
				t.Fatal(contents, err)
			}
		}
	}
}
