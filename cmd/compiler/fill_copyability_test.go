package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Copyability failures must stop before lowering and preserve existing output.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9;
// rules/compiler/semantic_ir.md — source validity before lowering.
func TestCLIFillCopyabilityStopsLowering(t *testing.T) {
	output := filepath.Join(t.TempDir(), "existing.ll")
	if err := os.WriteFile(output, []byte("existing user output"), 0600); err != nil {
		t.Fatal(err)
	}
	fixture := "../../testdata/sema/fill_copyability_invalid.sec"
	for _, args := range [][]string{{"sema", fixture}, {"check", fixture}, {"emit-ir", fixture}, {"emit-llvm", fixture, "-o", output}} {
		args = append(args, "--diagnostic-format=json")
		_, data, code := runCLIForDiagnostics(t, args...)
		if code == 0 {
			t.Fatal(args, data)
		}
		items := decodeOccurrenceDocument(t, data).Occurrences
		if len(items) != 5 {
			t.Fatal(args, items)
		}
		for _, item := range items {
			if item.ID == nil || *item.ID != "S1156" || item.Primary == nil {
				t.Fatal(args, item)
			}
		}
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "existing user output" {
		t.Fatal("fill errors changed existing output", err)
	}
}
