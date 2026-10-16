package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMapEntryIterationLSP presents key/value modes and unspecified order from
// Sema's canonical entry facts, and publishes the same invalid-header diagnostics.
// Rules: rules/collections/collections.md — §§14.2,14.6;
// rules/tooling/lsp.md — "Hover", "Diagnostics".
func TestMapEntryIterationLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/map_iteration_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "entries.sec"))
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, test := range []struct{ needle, key, value string }{{"for key, value", "key: EntryKey", "value: Payload"}, {"for ref key, ref mut value", "key: ref EntryKey", "value: ref mut Payload"}, {"for _, value", "key: discard (Resource)", "value: int"}, {"for key, _", "key: int", "value: discard (Resource)"}} {
		index := strings.Index(source, test.needle)
		if index < 0 {
			t.Fatal(test.needle)
		}
		for _, offset := range []int{index, index + strings.Index(test.needle, "key"), index + strings.Index(test.needle, "value")} {
			if offset < index {
				continue
			}
			hover, known := hoverForSource(uri, source, offsetPosition(source, offset))
			if !known || !strings.Contains(hover.Contents.Value, test.key) || !strings.Contains(hover.Contents.Value, test.value) || !strings.Contains(hover.Contents.Value, "Iteration order: unspecified") {
				t.Fatal(test.needle, hover, known)
			}
		}
	}
	data, err = os.ReadFile("../../testdata/sema/map_iteration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri = uriFromPath(filepath.Join(t.TempDir(), "invalid.sec"))
	errors := 0
	for _, d := range analyze(uri, string(data)) {
		if d.Severity == 1 {
			errors++
		}
	}
	if errors != 8 {
		t.Fatal("invalid map iteration diagnostic count", errors)
	}
}
