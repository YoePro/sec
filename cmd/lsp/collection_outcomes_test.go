package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCollectionOutcomePresentation keeps hover and synthetic definitions on
// the same canonical normal-absence versus resource-failure contract as Sema.
// Rules: rules/collections/collections.md — §§6.9,13.6–13.10,14.7,15.3,16;
// rules/compiler/compiler_known_members.md — "Synthetic definitions", "LSP".
func TestCollectionOutcomePresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/collection_outcomes_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "outcomes.sec"))
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, test := range []struct{ call, outcome string }{
		{"array.RemoveAt", "None, not an Error"},
		{"mapping.Remove", "None, not an Error"},
		{"items.Add", "Ok(false), not an Error"},
		{"values.Insert", "only structural/resource failure uses CollectionError"},
	} {
		offset := strings.Index(source, test.call) + strings.Index(test.call, ".") + 1
		position := offsetPosition(source, offset)
		hover, found := hoverForSource(uri, source, position)
		if !found || !strings.Contains(hover.Contents.Value, test.outcome) {
			t.Fatal(test.call, hover)
		}
		locations := definitionsForSource(uri, source, position)
		if len(locations) != 1 {
			t.Fatal(test.call, locations)
		}
		definition, found := compilerKnownDefinitionText(locations[0].URI)
		if !found || !strings.Contains(definition, test.outcome) {
			t.Fatal(test.call, definition)
		}
	}
}
