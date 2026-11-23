package main

import (
	"strings"
	"testing"
)

// Interface overload conformance selects the actual parameter signature when
// explaining mismatches, and retains canonical requirement source links.
// Rules: rules/declarations/interfaces.md §§5–6,12;
// rules/tooling/diagnostics.md — diagnostic cause and repair.
func TestInterfaceOverloadMatrixCLI(t *testing.T) {
	for _, name := range []string{"valid", "contracts_invalid", "declarations_invalid"} {
		_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/interface_overload_matrix/"+name+".sec", "--diagnostic-format=json")
		if name == "valid" {
			if code != 0 {
				t.Fatal(code, output)
			}
			continue
		}
		if code == 0 {
			t.Fatal("invalid interface accepted")
		}
		doc := decodeOccurrenceDocument(t, output)
		if name == "contracts_invalid" {
			if doc.Summary.Errors != 2 || !strings.Contains(output, "result type is int, but the interface requires bool") {
				t.Fatal(output)
			}
			for _, e := range doc.Occurrences {
				if e.ID == nil || len(e.Related) == 0 || len(e.Help) == 0 {
					t.Fatal(e)
				}
			}
		} else if doc.Summary.Errors != 3 {
			t.Fatal(output)
		}
	}
}
