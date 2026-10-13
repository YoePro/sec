package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
)

// TestLSPGenericExternABIBoundary preserves Sema's mandatory error and links
// on unsaved declarations, including parameters unused in the foreign signature.
// Rules: rules/compiler/monomorphization.md — §16.4;
// rules/tooling/lsp.md — Shared diagnostic model.
func TestLSPGenericExternABIBoundary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/generic_extern_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "extern.sec"))
	values := analyze(uri, string(data))
	count := 0
	for _, value := range values {
		if value.Code != diagnostics.UnresolvedGenericExtern {
			continue
		}
		count++
		if value.Severity != 1 || value.Data == nil || value.Data.ProofState != diagnostics.ProofInvalid || len(value.RelatedInformation) != 1 || value.RelatedInformation[0].Location.URI != uri || value.RelatedInformation[0].Message != "generic extern declaration" || value.Range.End.Character <= value.Range.Start.Character {
			t.Fatal(value)
		}
	}
	if count != 3 {
		t.Fatal(values)
	}
}
