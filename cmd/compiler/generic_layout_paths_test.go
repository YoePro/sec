package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestGenericStoragePathsCLI verifies the same mandatory owning proof and
// carrier navigation before every requested representation-dependent path.
// Rules: rules/memory/layout.md — §25(3), §42(4);
// rules/tooling/diagnostics.md — §§9,14.
func TestGenericStoragePathsCLI(t *testing.T) {
	path := "../../testdata/generics/storage_paths_invalid.sec"
	for _, command := range []string{"check", "sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		args := []string{command, path, "--diagnostic-format=json"}
		if strings.HasPrefix(command, "emit-") {
			args = append(args, "-o", "-")
		}
		_, stderr, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(command, code, stderr)
		}
		doc := decodeOccurrenceDocument(t, stderr)
		if doc.Summary.Errors != 7 {
			t.Fatal(doc)
		}
		for _, occurrence := range doc.Occurrences {
			if occurrence.ID == nil || (*occurrence.ID != diagnostics.RecursiveStructLayout && *occurrence.ID != diagnostics.RecursiveUnionLayout) || occurrence.Arguments["proof_state"] != "Invalid" || occurrence.Primary == nil || len(occurrence.Related) == 0 {
				t.Fatal(occurrence)
			}
		}
		if !strings.Contains(stderr, "by-value generic carrier Outer[U].inner") || !strings.Contains(stderr, "by-value generic carrier Inner[T].value") {
			t.Fatal(stderr)
		}
	}
}
