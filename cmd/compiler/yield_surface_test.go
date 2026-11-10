package main

import "testing"

// TestYieldSurfaceCLI checks the same exact yield contract at the CLI boundary.
// Rules: rules/concurrency/scheduling.md — §§7,11,22,55–56.
func TestYieldSurfaceCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/yield_surface_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/yield_surface_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 7 {
		t.Fatal(output)
	}
}
