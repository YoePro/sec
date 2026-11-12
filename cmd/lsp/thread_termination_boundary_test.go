package main

import (
	"os"
	"strings"
	"testing"
)

// TestPortableThreadTerminationBoundaryLSP shares Sema's typed rejection and
// offers no synthetic termination completion/hover/navigation for owners or
// observers. Explicitly declared methods on other types remain navigable.
// Rules: rules/concurrency/threads.md — §§40(6),52–53;
// rules/tooling/lsp.md — Diagnostics, Completion, Hover, Navigation.
func TestPortableThreadTerminationBoundaryLSP(t *testing.T) {
	uri := "file:///thread-termination-boundary/main.sec"
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/thread_termination_boundary_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		errors := 0
		for _, d := range analyze(uri, source) {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code != "S1227" || !strings.Contains(d.Message, "RequestCancel()") || !strings.Contains(d.Message, "unsafe target-specific API") {
				t.Fatal(d)
			}
		}
		want := 0
		if mode == "invalid" {
			want = 9
		}
		if errors != want {
			t.Fatal(mode, errors)
		}
		needle := "worker.Terminate()"
		if mode == "valid" {
			needle = "control.Terminate()"
		}
		offset := strings.Index(source, needle) + strings.Index(needle, "Terminate") + 1
		pos := offsetPosition(source, offset)
		hover, hovered := hoverForSource(uri, source, pos)
		definitions := definitionsForSource(uri, source, pos)
		if mode == "invalid" {
			if hovered || len(definitions) != 0 {
				t.Fatal("fabricated termination symbol", hover, definitions)
			}
		} else if !hovered || !strings.Contains(hover.Contents.Value, "Terminate") || len(definitions) != 1 {
			t.Fatal("lost declared method", hover, definitions)
		}
	}
	data, err := os.ReadFile("../../testdata/lsp/thread_termination_boundary_completion.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, name := range []string{"worker", "observer"} {
		offset := strings.Index(source, name+".\n") + len(name) + 1
		items := completeSource(uri, source, offset)
		assertCompletionLabels(t, items, []string{"ID", "Name", "Status"})
		for _, item := range items {
			if item.Label == "Terminate" || item.Label == "platform" {
				t.Fatal("legacy termination completion", item)
			}
		}
	}
}
