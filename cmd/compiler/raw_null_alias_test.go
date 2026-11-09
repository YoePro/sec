package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// CLI null-alias rejection keeps the canonical diagnostic, alias declaration
// location and help, and no rejected source produces an IR artifact.
// Rules: rules/memory/raw_pointers.md §§7(8),34(2–4),35.
func TestRawNullAliasCompilerDiagnostics(t *testing.T) {
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "out")
		args := []string{command, "../../testdata/sema/raw_null_alias_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 8 {
			t.Fatal(output)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || *d.ID != diagnostics.RawPointerNullDereference || len(d.Related) != 1 || len(d.Help) != 1 {
				t.Fatalf("lost null proof metadata: %+v", d)
			}
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid source emitted artifact", err)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/raw_null_alias_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
}
