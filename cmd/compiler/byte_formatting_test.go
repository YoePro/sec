package main

import "testing"

// TestByteRadixFormattingCLI checks the actual trusted core overload and all
// four selector members through ordinary public compiler source assembly.
// Rules: rules/library/core-library.md — §8.4;
// rules/compiler/compiler_known_members.md — §69.1 (MD-091).
func TestByteRadixFormattingCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/tooling/byte_formatting/members.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatalf("check failed (%d): %s", code, output)
	}
	if document := decodeOccurrenceDocument(t, output); len(document.Occurrences) != 0 {
		t.Fatal(document.Occurrences)
	}
}
