package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"testing"
)

// The editor consumes Sema's copied-null proof and canonical source links;
// it does not maintain its own pointer alias classifier.
// Rules: rules/memory/raw_pointers.md §§34–35.
func TestRawNullAliasLSPDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{
		{"raw_null_alias_invalid.sec", 8}, {"raw_null_alias_valid.sec", 0},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.name)
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
			if d.Code != diagnostics.RawPointerNullDereference || len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Location.URI != uri {
				t.Fatalf("lost alias diagnostic: %+v", d)
			}
		}
		if count != tc.count {
			t.Fatalf("%s: errors %d, want %d", tc.name, count, tc.count)
		}
	}
}
