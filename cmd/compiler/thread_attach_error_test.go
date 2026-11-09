package main

import (
	"strings"
	"testing"
)

// TestThreadAttachErrorCLI verifies ordinary typed error use and rejects the
// removed variant through compiler diagnostics.
// Rules: rules/concurrency/thread_local.md — §29(1–3).
func TestThreadAttachErrorCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_attach_error_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_attach_error_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 1 || !strings.Contains(output, "ThreadAttachError.NotAttached has never been declared") {
		t.Fatal(output)
	}
}
