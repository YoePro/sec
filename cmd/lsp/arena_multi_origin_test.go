package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArenaMultiOriginLSP uses the shared semantic join and validity producer
// for unsaved documents, including causal links to either possible origin.
// Rules: rules/memory/lifetime_analysis.md — §11.2;
// rules/tooling/lsp.md — Shared diagnostic model.
func TestArenaMultiOriginLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "origins.sec"))
	for _, suffix := range []string{"invalid", "valid"} {
		data, err := os.ReadFile("../../testdata/sema/arena_multi_origin_" + suffix + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze(uri, string(data))
		want := 0
		if suffix == "invalid" {
			want = 9
		}
		if len(values) != want {
			t.Fatal(suffix, values)
		}
		for _, value := range values {
			if value.Severity != 1 || len(value.RelatedInformation) != 1 || value.RelatedInformation[0].Location.URI != uri {
				t.Fatal(value)
			}
		}
	}
}
