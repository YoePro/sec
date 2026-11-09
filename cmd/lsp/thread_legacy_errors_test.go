package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadLegacyErrorsLSP removes stale portable completions and synthetic
// hover, retaining canonical attachment errors and project-declared identities.
// Rules: rules/concurrency/threads.md — §§3,71(7);
// rules/concurrency/thread_local.md — §§29,37.
func TestThreadLegacyErrorsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_legacy_errors_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///thread-legacy-errors/main.sec"
	offset := strings.Index(source, "ThreadSchedulingError") + len("Thread")
	labels := map[string]bool{}
	for _, item := range completeSource(uri, source, offset) {
		labels[item.Label] = true
	}
	if !labels["ThreadAttachError"] {
		t.Fatal(labels)
	}
	for _, name := range []string{"ThreadSchedulingError", "ThreadTerminationError", "ThreadContextError"} {
		if labels[name] {
			t.Fatal("legacy completion", name)
		}
		offset := strings.Index(source, name)
		if hover, ok := hoverForSource(uri, source, offsetPosition(source, offset)); ok {
			t.Fatal("legacy hover", name, hover)
		}
	}
	count := 0
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
	for _, name := range []string{"valid", "local"} {
		data, err := os.ReadFile("../../testdata/sema/thread_legacy_errors_" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range analyze(uri, string(data)) {
			if d.Severity == 1 {
				t.Fatal(d)
			}
		}
	}
}
