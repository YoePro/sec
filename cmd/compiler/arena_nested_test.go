package main

import (
	"sec/internal/diagnostics"
	"testing"
)

// CLI checks and analysis share mandatory parent-domain invalidation and
// borrowed-child escape errors with precise child declaration provenance.
// Rules: rules/memory/arena.md §§10,36,43–44,51,119.
func TestNestedArenaCompiler(t *testing.T) {
	for _, command := range []string{"check", "analyse"} {
		_, output, code := runCLIForDiagnostics(t, command, "../../testdata/sema/arena_nested_invalid.sec", "--diagnostic-format=json")
		if code != 3 {
			t.Fatalf("%s: code %d: %s", command, code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 8 {
			t.Fatalf("%s: %s", command, output)
		}
		for _, d := range doc.Occurrences {
			wantRelated := 2
			if d.ID != nil && *d.ID == diagnostics.BorrowOwnershipEndConflict {
				wantRelated = 1
			}
			if d.ID == nil || (*d.ID != diagnostics.ArenaResetLiveDependency && *d.ID != diagnostics.ArenaReleaseLiveDependency && *d.ID != diagnostics.BorrowOwnershipEndConflict) || len(d.Help) == 0 || len(d.Related) != wantRelated || d.Primary == nil {
				t.Fatalf("incomplete child dependency: %+v", d)
			}
		}
		_, output, code = runCLIForDiagnostics(t, command, "../../testdata/sema/arena_nested_valid.sec", "--diagnostic-format=json")
		if code != 0 {
			t.Fatalf("%s rejected released/scoped children: %s", command, output)
		}
	}
}
