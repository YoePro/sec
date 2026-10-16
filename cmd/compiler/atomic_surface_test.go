package main

import "testing"

// TestAtomicSurfaceCLI loads the real core enum and applies the canonical
// names and semantic type eligibility through ordinary compiler checks.
// Rules: rules/concurrency/atomics.md — §§4–9,12–13,43–45.
func TestAtomicSurfaceCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"atomic_surface_valid.sec", 0, 0}, {"atomic_surface_invalid.sec", 3, 15},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(test.file, code, stderr)
		}
		if test.errors > 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
	}
}
