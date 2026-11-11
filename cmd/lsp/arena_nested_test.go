package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
)

// Unsaved buffers use compiler-owned child/parent domain dependencies.
// Rules: rules/memory/arena.md §§36,43–44,51,119;
// rules/tooling/lsp.md §10.1 shared diagnostic model.
func TestNestedArenaLSP(t *testing.T) {
	for _, tc := range []struct {
		file  string
		count int
	}{{"arena_nested_invalid.sec", 8}, {"arena_nested_valid.sec", 0}} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			wantRelated := 2
			if d.Code == diagnostics.BorrowOwnershipEndConflict {
				wantRelated = 1
			}
			if (d.Code != diagnostics.ArenaResetLiveDependency && d.Code != diagnostics.ArenaReleaseLiveDependency && d.Code != diagnostics.BorrowOwnershipEndConflict) || len(d.RelatedInformation) != wantRelated || d.RelatedInformation[0].Location.URI != uri {
				t.Fatalf("incomplete child dependency: %+v", d)
			}
		}
		if count != tc.count {
			t.Fatalf("%s: want %d errors, got %d", tc.file, tc.count, count)
		}
	}
}
