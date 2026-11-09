package main

import "testing"

// TestChannelRevokeTypesCLI checks complete nested result/disposition typing
// through compiler diagnostics, including all three unavailable outcomes.
// Rules: rules/concurrency/channels.md — §§13–14.
func TestChannelRevokeTypesCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/channel_revoke_types_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/channel_revoke_types_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 4 {
		t.Fatal(output)
	}
}
