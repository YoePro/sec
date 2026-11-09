package main

import (
	"os"
	"strings"
	"testing"
)

// TestThreadMetadataBoundariesLSP uses canonical Kind field/enum facts and the
// metadata-only observer catalog, with no hidden Wait/select presentation.
// Rules: rules/concurrency/threads.md — §§23,40,43;
// rules/tooling/lsp.md — "Hover", "Completion".
func TestThreadMetadataBoundariesLSP(t *testing.T) {
	uri := "file:///thread-metadata/main.sec"
	for _, area := range []string{"thread_termination", "thread_observer_wait"} {
		data, err := os.ReadFile("../../testdata/sema/" + area + "_valid.sec")
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, d := range analyze(uri, source) {
			if d.Severity == 1 {
				t.Fatal(d)
			}
		}
		if area == "thread_termination" {
			offset := strings.Index(source, "value.Kind") + len("value.")
			hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
			if !ok || !strings.Contains(hover.Contents.Value, "ThreadTerminationKind") {
				t.Fatal(hover, ok)
			}
			completion := source + "\nfn Complete() void {\n discard ThreadTerminationKind.\n}\n"
			offset = strings.LastIndex(completion, "ThreadTerminationKind.") + len("ThreadTerminationKind.")
			assertCompletionLabels(t, completeSource(uri, completion, offset), []string{"Requested", "External", "Fault", "Unknown"})
		} else {
			completion := source + "\nfn Complete(observer: ThreadObserver[int]) void {\n discard observer.\n}\n"
			offset := strings.LastIndex(completion, "observer.") + len("observer.")
			labels := map[string]bool{}
			for _, item := range completeSource(uri, completion, offset) {
				labels[item.Label] = true
			}
			if !labels["ID"] || !labels["Name"] || !labels["Status"] || labels["Wait"] || labels["Start"] || labels["RequestCancel"] {
				t.Fatal(labels)
			}
		}
		invalid, err := os.ReadFile("../../testdata/sema/" + area + "_invalid.sec")
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
			t.Fatal(area, count)
		}
	}
}
