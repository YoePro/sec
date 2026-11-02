package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// Static bodies cannot obtain an instance from their enclosing type scope.
// The public CLI preserves registered diagnostics and accepts explicit receivers.
// Rules: rules/declarations/static.md — §§9,9.1;
// rules/tooling/diagnostics.md — §§2(6–10),14.
func TestStaticReceiverIsolationCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"receiver_invalid", "receiver_valid"} {
		data, err := os.ReadFile("../../testdata/sema/static_receiver/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, output, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name == "receiver_valid" {
			if code != 0 {
				t.Fatal(code, output)
			}
			continue
		}
		document := decodeOccurrenceDocument(t, output)
		if code != 3 || stdout != "" || document.Summary.Errors != 13 || len(document.Occurrences) != 13 {
			t.Fatal(code, stdout, output)
		}
		counts := map[string]int{}
		for _, e := range document.Occurrences {
			if e.ID == nil || *e.ID != diagnostics.StaticInstanceReceiverRequired {
				continue
			}
			if e.Name == nil || e.Unregistered || e.Severity != diagnostics.SeverityError || len(e.Help) == 0 || e.Primary == nil || e.Primary.Span.Start.Line == 0 {
				t.Fatal(e)
			}
			counts[*e.ID]++
		}
		if counts[diagnostics.StaticInstanceReceiverRequired] != 7 {
			t.Fatal(counts)
		}
	}
}
