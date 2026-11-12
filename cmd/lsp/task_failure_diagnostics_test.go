package main

import (
	"os"
	"strings"
	"testing"
)

// TestTaskFailureDomainDiagnosticsLSP shares typed compiler explanations and
// preserves both application Result and task-termination layers in hover.
// Rules: rules/concurrency/tasks.md — §§11–15,30(1),(8–9);
// rules/tooling/lsp.md — Diagnostics, Hover, One semantic source of truth.
func TestTaskFailureDomainDiagnosticsLSP(t *testing.T) {
	uri := "file:///task-failure-domains/main.sec"
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/task_failure_domains_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1234" && d.Code != "S1151" || !strings.Contains(d.Message, "Completed(Err(E))") || !strings.Contains(d.Message, "TaskSpawnError") || !strings.Contains(d.Message, "Panicked(PanicInfo)") {
				t.Fatal(d)
			}
		}
		want := 0
		if mode == "invalid" {
			want = 21
		}
		if count != want {
			t.Fatal(mode, count)
		}
	}
	data, err := os.ReadFile("../../testdata/lsp/task_failure_domains.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ name, detail string }{
		{"outcome", "TaskOutcome[Result[int, AppError]]"},
		{"creation", "no task was created"},
		{"execution", "after successful creation"},
		{"empty", "Completed carries no value"},
	} {
		offset := strings.Index(source, "discard "+tc.name) + len("discard ") + 1
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, tc.detail) || !strings.Contains(hover.Contents.Value, "Completed(Err(E))") || !strings.Contains(hover.Contents.Value, "Cancelled without a value") {
			t.Fatal(tc.name, hover, ok)
		}
	}
	// The type-name hover for the two error enums uses the same Sema text.
	for _, name := range []string{"TaskSpawnError", "TaskError"} {
		offset := strings.Index(source, name) + 1
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "Completed(Err(E))") {
			t.Fatal(name, hover, ok)
		}
	}
}
