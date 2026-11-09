package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadSettingLSP uses canonical Sema union types and variants for binding
// hover, completion and diagnostics; the editor adds no preference coercion.
// Rules: rules/concurrency/threads.md — §4;
// rules/tooling/lsp.md — "Hover", "Completion".
func TestThreadSettingLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_setting_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-setting/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(source, "match value") + len("match ")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "ThreadSetting[int]") {
		t.Fatal(hover, ok)
	}
	completion := source + "\nfn Complete() void {\n discard ThreadSetting.\n}\n"
	offset = strings.LastIndex(completion, "ThreadSetting.") + len("ThreadSetting.")
	assertCompletionLabels(t, completeSource(uri, completion, offset), []string{"Default", "Preferred", "Required"})
	invalid, err := os.ReadFile("../../testdata/sema/thread_setting_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
}
