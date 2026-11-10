package main

import (
	"os"
	"strings"
	"testing"
)

// TestCommandConfigurationErrorLSP consumes shared Sema enum facts for hover,
// completion and invalid-variant diagnostics.
// Rules: rules/concurrency/processes.md — §5.9; rules/concurrency/ipc.md — §97;
// rules/tooling/lsp.md — Hover, Completion, Diagnostics.
func TestCommandConfigurationErrorLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/ipc/command_configuration/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///command-configuration/main.sec"
	for _, d := range analyze(uri, text) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(text, "CommandConfigurationError")
	hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "enum CommandConfigurationError error") || !strings.Contains(hover.Contents.Value, "InvalidIOResource") {
		t.Fatal(hover, ok)
	}
	completion := text + "\nfn Complete() void {\n discard CommandConfigurationError.\n}\n"
	items := completeSource(uri, completion, strings.LastIndex(completion, "CommandConfigurationError.\n")+len("CommandConfigurationError."))
	assertCompletionLabels(t, items, []string{"InvalidState", "InvalidWorkingDirectory", "NotFound", "NotDirectory", "PermissionDenied", "InvalidEnvironmentName", "InvalidEnvironmentValue", "InvalidIOResource", "OutOfMemory"})
	for _, item := range items {
		if item.Label == "Unsupported" || item.Label == "NativeFailure" {
			t.Fatal(items)
		}
	}
	data, err = os.ReadFile("../../testdata/sema/ipc/command_configuration/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(data)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
}
