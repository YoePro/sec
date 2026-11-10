package main

import (
	"os"
	"strings"
	"testing"
)

// TestPropertyVisibilityLSP publishes canonical access failures with navigable
// property declarations; access within the owner and its extension stays valid.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.1–12.3;
// rules/tooling/lsp.md — Diagnostics; rules/tooling/diagnostics.md — §§8–12.
func TestPropertyVisibilityLSP(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{{"valid.sec", 0}, {"owner_invalid.sec", 6}} {
		data, err := os.ReadFile("../../testdata/sema/property_visibility/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///property-visibility/main.sec"
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1216" || !strings.Contains(d.Message, "not visible") || len(d.RelatedInformation) != 1 || d.RelatedInformation[0].Message != "property declared here" || d.RelatedInformation[0].Location.URI != uri {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
