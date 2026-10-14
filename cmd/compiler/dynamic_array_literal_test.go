package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Invalid owner literals must stop the compiler before IR or output writes,
// carrying the same structured diagnostic as the frontend.
// Rules: rules/collections/collections.md §5.6a;
// rules/compiler/semantic_ir.md — source validity before lowering.
func TestCLIDynamicArrayLiteralStopsLowering(t *testing.T) {
	output := filepath.Join(t.TempDir(), "existing.ll")
	if err := os.WriteFile(output, []byte("existing user output"), 0600); err != nil {
		t.Fatal(err)
	}
	fixture := "../../testdata/sema/dynamic_array_literal_invalid.sec"
	for _, args := range [][]string{
		{"sema", fixture}, {"check", fixture}, {"emit-ir", fixture},
		{"emit-llvm", fixture, "-o", output}, {"build", fixture, "-o", output},
	} {
		args = append(args, "--diagnostic-format=json")
		_, data, code := runCLIForDiagnostics(t, args...)
		if code == 0 {
			t.Fatal(args, data)
		}
		items := decodeOccurrenceDocument(t, data).Occurrences
		if len(items) != 1 || items[0].ID == nil || *items[0].ID != "S1155" || items[0].Primary == nil || items[0].Primary.Span.Start.Line != 4 {
			t.Fatal(args, items)
		}
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "existing user output" {
		t.Fatal("invalid literal changed existing output", err)
	}
}
