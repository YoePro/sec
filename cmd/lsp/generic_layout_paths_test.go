package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestLSPGenericStoragePaths retains the owning invalid proof and navigation
// through generic storage carriers in an unsaved editor document.
// Rules: rules/memory/layout.md — §25(3), §42(4);
// rules/tooling/lsp.md — Shared diagnostic model.
func TestLSPGenericStoragePaths(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/storage_paths_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "storage.sec"))
	values := analyze(uri, string(data))
	if len(values) != 7 {
		t.Fatal(values)
	}
	for _, value := range values {
		if (value.Code != diagnostics.RecursiveStructLayout && value.Code != diagnostics.RecursiveUnionLayout) || value.Severity != 1 || value.Data == nil || value.Data.ProofState != diagnostics.ProofInvalid || len(value.RelatedInformation) == 0 {
			t.Fatal(value)
		}
		for _, related := range value.RelatedInformation {
			if related.Location.URI != uri || related.Location.Range.End.Character <= related.Location.Range.Start.Character {
				t.Fatal(related)
			}
		}
		if strings.Contains(value.Message, "Wrapped[T]") && (len(value.RelatedInformation) != 3 || !strings.Contains(value.RelatedInformation[0].Message, "Outer[U].inner")) {
			t.Fatal(value)
		}
	}
}
