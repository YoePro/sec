package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP publishes compiler identities/ranges/help rather than rechecking receiver
// contracts or conformance placement. Rules: rules/declarations/interfaces.md §12; rules/tooling/lsp.md — diagnostics.
func TestInterfaceDiagnosticIDsLSP(t *testing.T) {
	for _, name := range []string{"type_placement_invalid", "extends_placement_invalid", "separate_placement_invalid", "receiver_invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/interface_diagnostics/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-interface-identities/main.sec", string(data))
		if name == "valid" {
			if len(values) != 0 {
				t.Fatal(values)
			}
			continue
		}
		want := diagnostics.ParserInterfaceConformancePlacement
		count := 1
		if name == "receiver_invalid" {
			want = diagnostics.InterfaceReceiverCapability
			count = 2
		}
		if len(values) != count {
			t.Fatalf("%s: %+v", name, values)
		}
		for _, value := range values {
			if value.Code != want || value.Severity != 1 || !strings.Contains(value.Message, "help:") || value.Range.End.Character <= value.Range.Start.Character {
				t.Fatalf("%s: %+v", name, value)
			}
			if strings.Contains(value.Message, "Increment") && len(value.RelatedInformation) != 1 {
				t.Fatalf("lost receiver declaration: %+v", value)
			}
		}
	}
}
