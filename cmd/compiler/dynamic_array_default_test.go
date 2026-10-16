package main

import "testing"

// TestEmptyDynamicArrayDefaultCLI accepts omitted mutable owners and rejects
// omitted immutable owners and safe slices through the public compiler path.
// Rules: rules/collections/collections.md — §6.2;
// rules/types/default_values.md — "Slices", "Immutable declarations".
func TestEmptyDynamicArrayDefaultCLI(t *testing.T) {
	for _, test := range []struct {
		suffix       string
		code, errors int
	}{{"valid", 0, 0}, {"invalid", 3, 2}} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/dynamic_array_default_"+test.suffix+".sec", "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(code, stderr)
		}
		if test.errors > 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
	}
}
