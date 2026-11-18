package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreationLifecycleCLI rejects an unresolved creation owner before emission
// and publishes the same registered, source-mapped mentor diagnostic as Sema.
// Rules: tasks.md §30; threads.md §34; diagnostics.md §§8–9,14.
func TestCreationLifecycleCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/creation_lifecycle_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/raw_spawn_result_scope_bug.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 2 || len(doc.Occurrences) != 2 {
		t.Fatal(output)
	}
	for _, item := range doc.Occurrences {
		if item.ID == nil || *item.ID != "S1247" {
			t.Fatal(output)
		}
	}
	if !strings.Contains(output, "Creation failure produces no child") {
		t.Fatal(output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid lifecycle emitted output", err)
	}
}
