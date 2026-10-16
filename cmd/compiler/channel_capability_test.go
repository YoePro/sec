package main

import (
	"strings"
	"testing"
)

// TestChannelCapabilityCLI loads the canonical core declaration and rejects
// non-v0.1 capability names through the normal semantic diagnostic path.
// Rules: rules/concurrency/channels.md — §§4–5.
func TestChannelCapabilityCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/channel_capability_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/channel_capability_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, stderr)
	}
	document := decodeOccurrenceDocument(t, stderr)
	if document.Summary.Errors != 1 || !strings.Contains(document.Occurrences[0].Message.Text, "ChannelCapability.Priority has never been declared") {
		t.Fatal(stderr)
	}
}
