package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMapAssignmentSurfaceCLI keeps declaration operators and Set from reaching
// lowering, with structured diagnostics and no replacement of existing output.
// Rules: rules/collections/collections.md — §14.5;
// rules/compiler/parser_recovery.md — "Error blocks code generation".
func TestMapAssignmentSurfaceCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/map_assignment_surface_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	for _, test := range []struct {
		file string
		code int
	}{{"parser/indexed_declaration_invalid.sec", 2}, {"sema/map_set_invalid.sec", 3}} {
		path := filepath.Join(t.TempDir(), "existing.ir")
		if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/"+test.file, "-o", path, "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, stderr)
		if code != test.code || stdout != "" || doc.Summary.Errors != 4 {
			t.Fatal(test.file, code, stdout, stderr)
		}
		for _, d := range doc.Occurrences {
			if d.ID == nil || d.Unregistered || d.Primary == nil || len(d.Help) == 0 {
				t.Fatal(d)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "existing output" {
			t.Fatal(string(data), err)
		}
	}
}
