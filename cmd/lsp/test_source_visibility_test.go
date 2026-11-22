package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTestSourceHelperVisibilityLSP keeps test body diagnostics and ordinary
// source definition navigation aligned with Sema's source-file identity.
// Rules: rules/tooling/testing.md — §§7.3–7.4,42.4;
// rules/tooling/lsp.md — Diagnostics, Navigation.
func TestTestSourceHelperVisibilityLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/test_source_visibility_test.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "helpers_test.sec"))
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, name := range []string{"_sharedTest", "__privateTest"} {
		pos := offsetPosition(source, strings.LastIndex(source, name+"()"))
		locations := definitionsForSource(uri, source, pos)
		decl := offsetPosition(source, strings.Index(source, name+"()"))
		if len(locations) != 1 || locations[0].URI != uri || locations[0].Range.Start != decl {
			t.Fatalf("%s definition: %+v, want %s %+v", name, locations, uri, decl)
		}
	}
}
