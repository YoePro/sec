package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// The CLI rejects static functions outside direct impl/interface member contexts
// and accepts ordinary static-duration module bindings without a static keyword.
// Rules: rules/declarations/static.md — §§3–4,9;
// rules/tooling/diagnostics.md — §§2(6–10),14.
func TestStaticDeclarationContextsCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"declarations_invalid", "declarations_valid"} {
		data, err := os.ReadFile("../../testdata/sema/static_context/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, output, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name == "declarations_valid" {
			if code != 0 {
				t.Fatal(code, output)
			}
			continue
		}
		document := decodeOccurrenceDocument(t, output)
		if code != 3 || stdout != "" || document.Summary.Errors != 5 || len(document.Occurrences) != 5 {
			t.Fatal(code, stdout, output)
		}
		counts := map[string]int{}
		for _, e := range document.Occurrences {
			if e.ID == nil || *e.ID != diagnostics.StaticFunctionContext {
				t.Fatal(e)
			}
			if e.Name == nil || e.Unregistered || e.Severity != diagnostics.SeverityError || len(e.Help) == 0 || e.Primary == nil || e.Primary.Span.Start.Line == 0 {
				t.Fatal(e)
			}
			counts[*e.ID]++
		}
		if counts[diagnostics.StaticFunctionContext] != 5 {
			t.Fatal(counts)
		}
	}
}
