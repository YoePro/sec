package main

import "testing"

// TestAtomicCoreIdentityCLI verifies the opaque core family through ordinary
// loader-backed checking, including construction, arity and source authority.
// Rules: rules/concurrency/atomics.md — §§4,7,11–13,31–32,45;
// rules/declarations/impl.md — §§2–3.
func TestAtomicCoreIdentityCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"atomic_identity_valid.sec", 0, 0},
		{"atomic_identity_invalid.sec", 3, 14},
		{"atomic_identity_spoof_invalid.sec", 3, 1},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(test.file, code, stderr)
		}
		if test.errors != 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
	}
}
