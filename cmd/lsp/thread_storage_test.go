package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadStorageLSP projects the non-generic noCopy identity and immutable
// capacity from compiler facts, without publishing private backing fields.
// Rules: rules/concurrency/threads.md — §8(1–7); rules/tooling/lsp.md — Hover, Completion.
func TestThreadStorageLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_storage_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-storage/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(source, "storage.Capacity") + len("storage.") + 1
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "property Capacity: uint") || !strings.Contains(hover.Contents.Value, "@noCopy") {
		t.Fatal(hover)
	}
	completion := source[:strings.Index(source, "storage.Capacity")] + "storage.\n}\n"
	cursor := strings.LastIndex(completion, "storage.") + len("storage.")
	items := completeSource(uri, completion, cursor)
	assertCompletionLabels(t, items, []string{"Capacity"})
	for _, item := range items {
		if item.Label == "_capacity" {
			t.Fatal("private capacity exposed", item)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/thread_storage_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count, capacityErrors := 0, 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
			if d.Code == "S1215" {
				capacityErrors++
			}
		}
	}
	if count != 11 || capacityErrors < 2 {
		t.Fatal(count, capacityErrors)
	}
}
