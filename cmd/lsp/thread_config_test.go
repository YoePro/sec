package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadConfigLSP exposes exactly the ordinary typed struct fields from
// Sema, and shares inline/ownership diagnostics rather than adding LSP coercions.
// Rules: rules/concurrency/threads.md — §§10–11; rules/tooling/lsp.md — Hover, Completion, Diagnostics.
func TestThreadConfigLSP(t *testing.T) {
	uri := "file:///thread-config/main.sec"
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"../../testdata/sema/thread_config_valid.sec", 0},
		{"../../testdata/sema/thread_config_invalid.sec", 10},
		{"../../testdata/parser/thread_inline_config_recovery.sec", 1},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity == 1 {
				count++
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
	data, err := os.ReadFile("../../testdata/lsp/thread_config_fields.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	offset := strings.Index(source, "config.Stack") + len("config.") + 1
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "ThreadSetting[uint]") {
		t.Fatal(hover, ok)
	}
	completion := source[:strings.Index(source, "config.Stack")] + "config.\n}\n"
	cursor := strings.LastIndex(completion, "config.") + len("config.")
	assertCompletionLabels(t, completeSource(uri, completion, cursor), []string{"Name", "Stack", "Affinity", "Priority", "Start", "Storage"})
}
