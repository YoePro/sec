package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// TestLSPCollectionGrowth consumes the same allocation proof in unsaved editor
// diagnostics, including the transitive introducing operation.
// Rules: rules/memory/allocation.md — §§24(4),29(1),(4).
func TestLSPCollectionGrowth(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/collection_growth_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "growth.sec"))
	count := 0
	for _, value := range analyze(uri, string(data)) {
		if value.Severity != 1 {
			continue
		}
		count++
		if value.Code != diagnostics.NoAllocViolation || len(value.RelatedInformation) == 0 || value.RelatedInformation[0].Location.URI != uri {
			t.Fatal(value)
		}
	}
	if count != 11 {
		t.Fatalf("got %d errors; want 11", count)
	}
}
