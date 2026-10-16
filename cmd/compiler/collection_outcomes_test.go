package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCollectionNormalOutcomesCLI accepts normal absence as bool/Option and
// rejects conflating it with CollectionError before attempting any lowering.
// Rules: rules/collections/collections.md — §§6.9,13.6–13.10,14.7,15.3,16.
func TestCollectionNormalOutcomesCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/collection_outcomes_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	path := filepath.Join(t.TempDir(), "output.ir")
	if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/collection_outcomes_invalid.sec", "-o", path, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || stdout != "" || doc.Summary.Errors != 5 {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing output" {
		t.Fatal(string(data), err)
	}
}
