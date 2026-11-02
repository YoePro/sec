package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// A failed source check retains MD-120 identity, severity, source ranges and
// repairs in real CLI JSON, without relying on draft core-library contracts.
// Rules: rules/tooling/diagnostics.md — §§2(6–10),14; MD-120 §10(7).
func TestGenericErrorDiagnosticIDsCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/generic_error_diagnostics/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, output, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if name == "valid" {
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
			if e.ID == nil || e.Name == nil || e.Unregistered || e.Severity != diagnostics.SeverityError || len(e.Help) == 0 || e.Primary == nil || e.Primary.Span.Start.Line == 0 {
				t.Fatal(e)
			}
			counts[*e.ID]++
		}
		if counts[diagnostics.InvalidResultErrorChannel] != 2 || counts[diagnostics.GenericInterfaceArgumentCount] != 2 || counts[diagnostics.ErrorVariantRequiresConcreteChannel] != 9 {
			t.Fatal(counts)
		}
	}
}
