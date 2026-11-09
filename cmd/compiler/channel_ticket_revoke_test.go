package main

import (
	"strings"
	"testing"
)

// TestTicketOwnedRevokeCLI preserves consuming-ticket diagnostics, including
// rejection of the legacy sender-owned operation without consuming its ticket.
// Rules: rules/concurrency/channels.md — §§18,30,57(6).
func TestTicketOwnedRevokeCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/ticket_revoke_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/ticket_revoke_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 6 || !strings.Contains(output, "ticket.Revoke()") {
		t.Fatal(output)
	}
}
