package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSelectProcessObserverLSP shares accepted operation identity and signature,
// metadata-only completion/navigation, and unsaved select diagnostics with Sema.
// Rules: rules/concurrency/select.md §§25–26,54,61; rules/concurrency/processes.md §5.14;
// rules/tooling/lsp.md — Hover, Completion, Navigation and diagnostics.
func TestSelectProcessObserverLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "observer.sec"))
	source, err := os.ReadFile("../../testdata/sema/select_observer/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, d := range analyze(uri, text) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(text, "copied.Wait") + len("copied.")
	pos := offsetPosition(text, offset)
	hover, ok := hoverForSource(uri, text, pos)
	if !ok || !strings.Contains(hover.Contents.Value, "Wait() ProcessStatus") || !strings.Contains(hover.Contents.Value, "non-owning") {
		t.Fatal(hover, ok)
	}
	locations := definitionsForSource(uri, text, pos)
	if len(locations) != 1 {
		t.Fatal(locations)
	}
	definition, ok := compilerKnownDefinitionText(locations[0].URI)
	if !ok || !strings.Contains(definition, "fn Wait() ProcessStatus") || !strings.Contains(definition, "rules/concurrency/processes.md") {
		t.Fatal(definition, ok)
	}
	completion := text + "\nfn Complete(observer: ProcessObserver) void { discard observer. }\n"
	offset = strings.LastIndex(completion, "observer.") + len("observer.")
	labels := map[string]bool{}
	for _, item := range completeSource(uri, completion, offset) {
		labels[item.Label] = true
	}
	for _, name := range []string{"ID", "Name", "Status", "Wait"} {
		if !labels[name] {
			t.Fatal(labels)
		}
	}
	for _, name := range []string{"Value", "CompletionError", "Panic", "Termination", "Start", "Terminate", "Observe", "RequestCancel"} {
		if labels[name] {
			t.Fatal("owner operation on observer", labels)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/select_observer/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 9 {
		t.Fatal(count)
	}
}
