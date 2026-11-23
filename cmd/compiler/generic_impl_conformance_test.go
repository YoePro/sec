package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestGenericImplConformanceCLI checks specialized full member contracts and
// preserves registered conformance errors through the public JSON frontend.
// Rules: rules/declarations/generics.md — §§8,14,22,25;
// rules/declarations/interfaces.md — §§4–6,9,12.
func TestGenericImplConformanceCLI(t *testing.T) {
	for _, name := range []string{"valid", "setters", "contract_invalid"} {
		t.Run(name, func(t *testing.T) {
			_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/generic_impl_conformance/"+name+".sec", "--diagnostic-format=json")
			document := decodeOccurrenceDocument(t, output)
			if name != "contract_invalid" {
				if code != 0 || len(document.Occurrences) != 0 {
					t.Fatalf("%d: %s", code, output)
				}
				return
			}
			if code == 0 || len(document.Occurrences) != 10 {
				t.Fatalf("%d: %s", code, output)
			}
			counts := map[string]int{}
			for _, item := range document.Occurrences {
				// The existing generic-constraint rejection has no registered ID;
				// identities below are mandatory for all nine interface member failures.
				if item.ID == nil && strings.Contains(item.Message.Text, "type Duck[int] does not satisfy constraint Contract[int]") {
					continue
				}
				if item.ID == nil || item.Unregistered || item.Primary == nil || item.Primary.Span.Start.Line == 0 {
					t.Fatalf("incomplete diagnostic: %+v", item)
				}
				counts[*item.ID]++
			}
			if counts[diagnostics.InterfaceMemberIncompatible] != 5 || counts[diagnostics.InterfaceMemberMissing] != 4 {
				t.Fatal(counts)
			}
		})
	}
}
