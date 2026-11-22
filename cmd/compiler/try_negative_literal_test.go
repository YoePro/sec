package main

import "testing"

// TestTryNegativeLiteralCLI accepts the representable literal argument and
// retains exactly one structured S1093 for a genuinely fallible argument.
// Rules: rules/errors/runtime_checks.md — §§5.1,7;
// rules/errors/errorhandling.md — §§9,11–13.
func TestTryNegativeLiteralCLI(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/try_negative_literal_"+mode+".sec", "--diagnostic-format=json")
		wantCode, wantCount := 0, 0
		if mode == "invalid" {
			wantCode, wantCount = 3, 1
		}
		doc := decodeOccurrenceDocument(t, output)
		if code != wantCode || len(doc.Occurrences) != wantCount {
			t.Fatal(code, output)
		}
	}
}
