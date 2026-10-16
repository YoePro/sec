package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShapedMultiplyExtentLSP consumes the same derived-shape validation for an
// unsaved source without implementing parallel shaped algebra in the editor.
// Rules: rules/collections/shaped-types.md — §§4–5,22,29.
func TestShapedMultiplyExtentLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/shaped_multiply_facts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "shapes.sec"))
	found := false
	for _, item := range analyze(uri, string(data)) {
		if strings.Contains(item.Message, "18446744073709551616 overflows target uint") {
			found = true
			if item.Severity != 1 || item.Range.Start.Line != 8 {
				t.Fatal(item)
			}
		}
	}
	if !found {
		t.Fatal("missing derived-shape diagnostic")
	}
}
