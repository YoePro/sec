package main

import (
	"os"
	"testing"
)

// TestPropertyEffectsLSP publishes the canonical accessor effect path rather
// than treating property syntax as a pure field observation or store.
// Rules: rules/declarations/properties.md — §14; rules/errors/panic.md — §21.
func TestPropertyEffectsLSP(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{{"valid.sec", 0}, {"callers_invalid.sec", 3}} {
		data, err := os.ReadFile("../../testdata/sema/property_effects/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		uri := "file:///property-effects/main.sec"
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1092" || len(d.RelatedInformation) == 0 || d.RelatedInformation[0].Location.URI != uri {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
