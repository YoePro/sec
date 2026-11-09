package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadAttachErrorLSP uses canonical Sema enum facts for hover, completion
// and diagnostics without a parallel TLS error inventory.
// Rules: rules/concurrency/thread_local.md — §§29,63–64;
// rules/tooling/lsp.md — "Hover".
func TestThreadAttachErrorLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_attach_error_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-attach-error/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(source, "ThreadAttachError]")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "AlreadyAttached") || !strings.Contains(hover.Contents.Value, "ResourceUnavailable") || !strings.Contains(hover.Contents.Value, "NativeFailure") || strings.Contains(hover.Contents.Value, "NotAttached") {
		t.Fatal(hover, ok)
	}
	offset = strings.Index(source, "ThreadAttachError.AlreadyAttached") + len("ThreadAttachError.")
	assertCompletionLabels(t, completeSource(uri, source, offset), []string{"AlreadyAttached", "ResourceUnavailable", "NativeFailure"})
	invalid, err := os.ReadFile("../../testdata/sema/thread_attach_error_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity != 1 {
			continue
		}
		count++
		if !strings.Contains(d.Message, "ThreadAttachError.NotAttached has never been declared") {
			t.Fatal(d)
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
