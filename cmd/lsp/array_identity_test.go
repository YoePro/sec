package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArrayOwnerIdentityLSP preserves frontend nominal/family decisions on
// unsaved source, including both flat and nested named fixed-array equality.
// Rules: rules/collections/collections.md — §§6.1,10–12;
// rules/tooling/lsp.md — "Diagnostics".
func TestArrayOwnerIdentityLSP(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{{"array_identity_valid.sec", 0}, {"array_identity_invalid.sec", 5}, {"array_nominal_equality_invalid.sec", 3}} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "identity.sec"))
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if test.file == "array_nominal_equality_invalid.sec" && d.Code != "S1019" {
				t.Fatal(d)
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
	}
}
