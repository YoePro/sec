package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStringEnumCTETooling uses actual Sema-computed values for hover/aliases,
// and retains unavailable execution diagnostics in unsaved editor source.
// Rules: rules/declarations/enums.md — §§5,7–8;
// rules/tooling/lsp.md — Hover, Diagnostics.
func TestStringEnumCTETooling(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/string_enum_cte/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, item := range analyze(uri, text) {
		if item.Severity == 1 {
			t.Fatal(item)
		}
	}
	for _, test := range []struct{ member, value string }{{"Primary", "Alpha Beta"}, {"Alias", "Alpha Beta"}, {"Empty", "\"\""}, {"Unicode", "å🙂"}} {
		at := strings.Index(text, test.member+" ")
		hover, ok := hoverForSource(uri, text, offsetPosition(text, at+1))
		if !ok || !strings.Contains(hover.Contents.Value, test.value) {
			t.Fatalf("%s: %v %+v", test.member, ok, hover)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/string_enum_cte/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range analyze(uri, string(invalid)) {
		if item.Code == "S1101" {
			found = true
			if item.Severity != 1 || !strings.Contains(item.Message, "requires compile-time execution") || !strings.Contains(item.Message, "help:") {
				t.Fatal(item)
			}
		}
	}
	if !found {
		t.Fatal("missing unavailable CTE diagnostic")
	}
}
