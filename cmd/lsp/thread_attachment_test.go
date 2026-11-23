package main

import (
	"os"
	"strings"
	"testing"
)

// Attachment tooling reads the compiler's exact signature and token Context
// property; no editor-owned legacy ThreadContext return is substituted.
// Rules: rules/concurrency/thread_local.md §§30–31,63–64.
func TestThreadAttachmentLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_attachment/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-attachment/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, item := range []struct{ needle, want string }{
		{"Thread.AttachCurrent", "Result[ThreadAttachment, ThreadAttachError]"},
		{"token.Context", "property Context: ThreadContext"},
	} {
		offset := strings.Index(source, item.needle) + strings.Index(item.needle, ".") + 2
		pos := offsetPosition(source, offset)
		h, ok := hoverForSource(uri, source, pos)
		if !ok || !strings.Contains(h.Contents.Value, item.want) {
			t.Fatal(item.needle, h, ok)
		}
		locations := definitionsForSource(uri, source, pos)
		if len(locations) != 1 {
			t.Fatal("missing canonical navigation", locations)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/thread_attachment/calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			errors++
		}
	}
	if errors != 7 {
		t.Fatalf("errors %d want7", errors)
	}
}
