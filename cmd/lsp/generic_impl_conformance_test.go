package main

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// TestGenericImplConformanceLSP uses compiler conformance rather than a separate
// editor-only generic surface and preserves requirement navigation on errors.
// Rules: rules/declarations/generics.md — §§8,14,22,25;
// rules/declarations/interfaces.md — §§4–6,9,12; rules/tooling/lsp.md — diagnostics.
func TestGenericImplConformanceLSP(t *testing.T) {
	for _, name := range []string{"valid", "setters", "contract_invalid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/sema/generic_impl_conformance/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			values := analyze("file:///tmp/sec-generic-conformance/"+name+".sec", string(data))
			if name != "contract_invalid" {
				if len(values) != 0 {
					t.Fatal(values)
				}
				return
			}
			if len(values) != 10 {
				t.Fatalf("%+v", values)
			}
			counts := map[string]int{}
			for _, value := range values {
				counts[value.Code]++
				if value.Severity != 1 || value.Range.End.Character <= value.Range.Start.Character {
					t.Fatalf("invalid diagnostic: %+v", value)
				}
				if value.Code == diagnostics.InterfaceMemberIncompatible || value.Code == diagnostics.InterfaceMemberMissing {
					if len(value.RelatedInformation) != 1 {
						t.Fatalf("missing requirement navigation: %+v", value)
					}
				}
			}
			if counts[diagnostics.InterfaceMemberIncompatible] != 5 || counts[diagnostics.InterfaceMemberMissing] != 4 {
				t.Fatal(counts)
			}
		})
	}
}
