package main

import (
	"os"
	"strings"
	"testing"
)

// TestTaskSpawnResultLSP uses Sema's raw creation result in diagnostics and
// binding hover, keeping creation separate from the awaited terminal outcome.
// Rules: rules/concurrency/tasks.md — §§4–5,30;
// rules/tooling/lsp.md — Diagnostics, Hover, One semantic source of truth.
func TestTaskSpawnResultLSP(t *testing.T) {
	uri := "file:///task-spawn-result/main.sec"
	for _, tc := range []struct {
		mode  string
		count int
	}{{"valid", 0}, {"invalid", 5}} {
		source, err := os.ReadFile("../../testdata/sema/task_spawn_result_" + tc.mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := 0
		for _, d := range analyze(uri, string(source)) {
			if d.Severity == 1 {
				errors++
			}
		}
		if errors != tc.count {
			t.Fatal(tc.mode, errors)
		}
		if tc.mode != "valid" {
			continue
		}
		text := string(source)
		for _, item := range []struct{ marker, name, detail string }{
			{"match creation", "creation", "Result[Task[int], TaskSpawnError]"},
			{"discard await worker", "worker", "Task[int]"},
		} {
			offset := strings.LastIndex(text, item.marker) + strings.Index(item.marker, item.name) + 1
			hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
			if !ok || !strings.Contains(hover.Contents.Value, item.detail) {
				t.Fatal(item.marker, hover, ok)
			}
		}
	}
}
