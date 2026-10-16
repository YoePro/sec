package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMapAssignmentSurfaceLSP exposes canonical syntax and map-member guidance
// on unsaved files without flagging declarations or an ordinary user Set method.
// Rules: rules/collections/collections.md — §14.5;
// rules/tooling/lsp.md — "Diagnostics".
func TestMapAssignmentSurfaceLSP(t *testing.T) {
	for _, test := range []struct {
		file, code string
		want       int
	}{{"parser/indexed_declaration_invalid.sec", "P2013", 4}, {"sema/map_set_invalid.sec", "S1157", 4}, {"sema/map_assignment_surface_valid.sec", "", 0}} {
		data, err := os.ReadFile("../../testdata/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "surface.sec"))
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != test.code || !strings.Contains(d.Message, "indexed") {
				t.Fatal(d)
			}
		}
		if count != test.want {
			t.Fatal(test.file, count)
		}
	}
}
