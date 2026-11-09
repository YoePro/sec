package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadLifecycleErrorsLSP projects exact compiler enum inventories into
// completion and hover, with canonical diagnostics for obsolete variants.
// Rules: rules/concurrency/threads.md — §§18–19;
// rules/tooling/lsp.md — "Hover", "Completion".
func TestThreadLifecycleErrorsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_lifecycle_errors_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-lifecycle-errors/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct {
		name, first string
		variants    []string
	}{
		{"ThreadSpawnError", "OutOfMemory", []string{"OutOfMemory", "ResourceLimit", "StackAllocationFailed", "InvalidConfiguration", "PermissionDenied", "ThreadLocalInitializationFailed", "NativeFailure"}},
		{"ThreadStartError", "InvalidState", []string{"InvalidState", "ResourceUnavailable", "PermissionDenied", "NativeFailure"}},
	} {
		offset := strings.Index(source, tc.name)
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || strings.Contains(hover.Contents.Value, "Unsupported") {
			t.Fatal(hover, ok)
		}
		for _, variant := range tc.variants {
			if !strings.Contains(hover.Contents.Value, variant) {
				t.Fatal(variant, hover)
			}
		}
		completion := source + "\nfn Complete() void {\n discard " + tc.name + ".\n}\n"
		offset = strings.LastIndex(completion, tc.name+".\n") + len(tc.name) + 1
		assertCompletionLabels(t, completeSource(uri, completion, offset), tc.variants)
	}
	invalid, err := os.ReadFile("../../testdata/sema/thread_lifecycle_errors_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity != 1 {
			continue
		}
		count++
		if !strings.Contains(d.Message, "has never been declared") {
			t.Fatal(d)
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
}
