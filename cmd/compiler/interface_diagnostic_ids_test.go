package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// The public compiler frontend preserves interface diagnostic IDs, repair help
// and locations in its JSON stream. Builtin-only sources isolate core migrations.
// Rules: rules/declarations/interfaces.md — §12; rules/tooling/diagnostics.md — §14.
func TestInterfaceDiagnosticIDsCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"type_placement_invalid", "extends_placement_invalid", "separate_placement_invalid", "receiver_invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/interface_diagnostics/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name+".sec")
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
		doc := decodeOccurrenceDocument(t, output)
		want := diagnostics.ParserInterfaceConformancePlacement
		count := 1
		if name == "receiver_invalid" {
			want = diagnostics.InterfaceReceiverCapability
			count = 2
		}
		if code == 0 || stdout != "" || doc.Summary.Errors != count || len(doc.Occurrences) != count {
			t.Fatal(name, code, stdout, output)
		}
		for _, e := range doc.Occurrences {
			if e.ID == nil || *e.ID != want || e.Name == nil || e.Unregistered || e.Severity != diagnostics.SeverityError || len(e.Help) == 0 || e.Primary == nil || e.Primary.Span.Start.Line == 0 {
				t.Fatal(name, e)
			}
		}
	}
}
