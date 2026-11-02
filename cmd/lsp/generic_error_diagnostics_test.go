package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// CLI and LSP must publish Sema's stable identities and practical repairs for
// invalid template channels, unknown variants and incomplete interface views.
// Rules: rules/tooling/diagnostics.md — §§2(6–10),15; MD-120 §10(7).
func TestGenericErrorDiagnosticIDsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_error_diagnostics/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	values := analyze("file:///tmp/sec-md120-diagnostics/main.sec", string(data))
	counts := map[string]int{}
	for _, d := range values {
		id := d.Code
		if id == "" {
			t.Fatalf("no registered identity: %+v", d)
		}
		counts[id]++
		if d.Severity != 1 || !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character {
			t.Fatalf("incomplete diagnostic: %+v", d)
		}
	}
	if len(values) != 13 || counts[diagnostics.InvalidResultErrorChannel] != 2 || counts[diagnostics.GenericInterfaceArgumentCount] != 2 || counts[diagnostics.ErrorVariantRequiresConcreteChannel] != 9 {
		t.Fatalf("diagnostics: %+v", values)
	}
}
