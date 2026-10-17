package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandIOModesLSP presents both exact non-error mode enums from shared
// core facts, including Resource documentation and real type/member navigation.
// Rules: rules/concurrency/ipc.md — §94;
// rules/concurrency/processes.md — §§5.12–5.13;
// rules/tooling/lsp.md — Hover, Completion and Go to definition.
func TestCommandIOModesLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/ipc/command_modes_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "modes.sec"))
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatal(diagnostic)
		}
	}
	for _, test := range []struct{ name, second, capability string }{
		{"CommandInputMode", "Closed", "PipeReader"},
		{"CommandOutputMode", "Discard", "PipeWriter"},
	} {
		offset := strings.Index(source, ") "+test.name) + 2
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "enum "+test.name+" {") || strings.Contains(hover.Contents.Value, "enum "+test.name+" error") {
			t.Fatal(test.name, hover, ok)
		}
		for _, variant := range []string{"Inherit", test.second, "Pipe", "Resource"} {
			if !strings.Contains(hover.Contents.Value, variant) {
				t.Fatal(hover)
			}
		}
		resource := strings.Index(source, test.name+".Resource") + len(test.name) + 1
		hover, ok = hoverForSource(uri, source, offsetPosition(source, resource))
		if !ok || !strings.Contains(hover.Contents.Value, test.name+".Resource") || !strings.Contains(hover.Contents.Value, "owned") || !strings.Contains(hover.Contents.Value, test.capability) {
			t.Fatal(hover, ok)
		}
		for _, index := range []int{offset, resource} {
			definitions := definitionsForSource(uri, source, offsetPosition(source, index))
			if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/command_io_modes.sec") {
				t.Fatal(definitions)
			}
		}
		completion := source + "\nfn Complete() void {\n    discard " + test.name + ".\n}\n"
		items := completeSource(uri, completion, strings.LastIndex(completion, test.name+".\n")+len(test.name)+1)
		assertCompletionLabels(t, items, []string{"Inherit", test.second, "Pipe", "Resource"})
		other := "Closed"
		if test.second == "Closed" {
			other = "Discard"
		}
		for _, item := range items {
			if item.Label == other || item.Label == "Null" || item.Label == "Handle" {
				t.Fatal(items)
			}
		}
	}
}
