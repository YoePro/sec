package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Rules: rules/tooling/lsp.md — Hover, One semantic source of truth.
func TestCollectionHoverUsesCompilerFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/collection_metadata.sec")
	if err != nil {
		t.Fatal(err)
	}
	text, uri := string(data), "file:///collection_metadata.sec"
	a := analyzeNavigationSource(uri, text)
	fn := a.Functions()["Inspect"][0]
	for _, param := range fn.Parameters {
		offset := strings.Index(text, param.Name+":")
		result, ok := hoverForSource(uri, text, offsetPosition(text, offset+1))
		if !ok {
			t.Fatalf("no hover for %s", param.Name)
		}
		facts, collection := sema.CollectionFactsOf(param.Type)
		if collection {
			if !strings.Contains(result.Contents.Value, facts.Display()) {
				t.Fatalf("hover differs from compiler: %+v; want %s", result, facts.Display())
			}
		} else if strings.Contains(result.Contents.Value, "Collection ") {
			t.Fatal(result)
		}
	}
}
