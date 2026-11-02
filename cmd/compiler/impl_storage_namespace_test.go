package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// CLI JSON retains registered conflicts and both declarations' source evidence.
// Rules: rules/declarations/impl.md — §23; rules/declarations/static.md — §6;
// rules/tooling/diagnostics.md — §§2,5–7,14.
func TestImplStorageNamespaceCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"conflicts_invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/impl_storage_namespace/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, output, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name == "valid" {
			if code != 0 {
				t.Fatal(code, output)
			}
			continue
		}
		document := decodeOccurrenceDocument(t, output)
		if code != 3 || stdout != "" || document.Summary.Errors != 54 || len(document.Occurrences) != 54 {
			t.Fatal(code, stdout, output)
		}
		counts := map[string]int{}
		for _, e := range document.Occurrences {
			if e.ID == nil || *e.ID != diagnostics.AssociatedStorageConflict {
				t.Fatal(e)
			}
			if e.Name == nil || e.Unregistered || e.Severity != diagnostics.SeverityError || len(e.Help) == 0 || e.Primary == nil || e.Primary.Span.Start.Line == 0 {
				t.Fatal(e)
			}
			counts[*e.ID]++
		}
		if counts[diagnostics.AssociatedStorageConflict] != 54 {
			t.Fatal(counts)
		}
	}
}
