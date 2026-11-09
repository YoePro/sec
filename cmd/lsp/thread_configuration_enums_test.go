package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadConfigurationEnumsLSP uses canonical compiler enum inventories for
// hover and completion, preserving their distinct non-error identities.
// Rules: rules/concurrency/threads.md — §§5–6;
// rules/tooling/lsp.md — "Hover", "Completion".
func TestThreadConfigurationEnumsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_configuration_enums_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-configuration/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct {
		name     string
		variants []string
	}{
		{"ThreadStartMode", []string{"Eager", "Deferred"}},
		{"ThreadPriority", []string{"Lowest", "Low", "Normal", "High", "Highest"}},
	} {
		offset := strings.Index(source, tc.name)
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok {
			t.Fatal(tc.name, hover)
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
	invalid, err := os.ReadFile("../../testdata/sema/thread_configuration_enums_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 4 {
		t.Fatal(count)
	}
}
