package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// LSP consumes the same origin/borrow failures and related source locations as
// Sema and CLI; no editor-owned pointer provenance model is introduced.
// Rules: rules/memory/raw_pointers.md §§34–35.
func TestRawOriginAliasLSPDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		errors int
	}{
		{"raw_origin_alias_invalid.sec", 4}, {"raw_origin_alias_valid.sec", 0},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.name)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		counts := map[string]int{}
		total := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			total++
			counts[d.Code]++
			if len(d.RelatedInformation) == 0 || d.RelatedInformation[0].Location.URI != uri {
				t.Fatalf("lost origin source: %+v", d)
			}
		}
		if total != tc.errors {
			t.Fatal(tc.name, total, counts)
		}
		if total > 0 && (counts[diagnostics.RawPointerDanglingOrigin] != 2 || counts[diagnostics.BorrowOwnerReadConflict] != 1 || counts[diagnostics.BorrowMutationConflict] != 1) {
			t.Fatal(counts)
		}
	}
}
