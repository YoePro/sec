package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP publishes the canonical cross-category conflicts with related locations.
// Rules: rules/declarations/impl.md — §23; rules/declarations/static.md — §6;
// rules/tooling/diagnostics.md — §§2,5–7,15.
func TestImplStorageNamespaceLSP(t *testing.T) {
	for _, name := range []string{"valid", "conflicts_invalid"} {
		data, err := os.ReadFile("../../testdata/sema/impl_storage_namespace/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-static-namespace/main.sec", string(data))
		errors, contexts := 0, 0
		for _, d := range values {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code == diagnostics.AssociatedStorageConflict {
				contexts++
				if !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character || len(d.RelatedInformation) == 0 {
					t.Fatalf("context diagnostic: %+v", d)
				}
			}
		}
		if name == "valid" && errors != 0 || name == "conflicts_invalid" && (errors != 54 || contexts != 54) {
			t.Fatalf("%s: %+v", name, values)
		}
	}
}
