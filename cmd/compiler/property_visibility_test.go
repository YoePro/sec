package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPropertyVisibilityCLI retains one registered error per inaccessible
// access, with a declaration witness, before native/IR artifact production.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.1–12.3.
func TestPropertyVisibilityCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/property_visibility/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "private.ir")
		args := []string{command, "../../testdata/sema/property_visibility/owner_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 6 {
			t.Fatal(output)
		}
		if strings.Count(output, `"id": "S1216"`) != 6 || !strings.Contains(output, "property declared here") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("inaccessible properties emitted IR", err)
		}
	}
}
