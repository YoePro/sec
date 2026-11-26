package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Public checking uses the retired-member mentor and rejects invalid output;
// the replacement's complete StringView API is accepted without source rewrites.
// Rules: rules/compiler/compiler_known_members.md §21; rules/tooling/diagnostics.md §14.
func TestStringSliceMigrationCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/string_ranges/registry_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/string_ranges/slice_removed_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	document := decodeOccurrenceDocument(t, output)
	if document.Summary.Errors != 5 || !strings.Contains(output, "string.Slice has been removed") || !strings.Contains(output, "borrowed StringView") {
		t.Fatal(output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid artifact", err)
	}
}
