package main

import (
	"os"
	"strings"
	"testing"
)

// TestIteratorHoverUsesCompilerResolution verifies identical compiler-owned
// descriptions on loop syntax, binding and source, including generic templates.
// Rules: rules/compiler/semantic_ir.md — §109(1–4),§115(2).
func TestIteratorHoverUsesCompilerResolution(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/iterator_resolution.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///iterator_resolution.sec"
	analyzer := analyzeNavigationSource(uri, text)
	facts := analyzer.IteratorResolutions()
	local := facts[:0]
	for _, fact := range facts {
		if fact.Source.File == pathFromURI(uri) {
			local = append(local, fact)
		}
	}
	facts = local
	if len(facts) != 3 {
		t.Fatalf("facts: %+v", facts)
	}
	for _, fact := range facts {
		for _, token := range []struct{ line, column int }{{fact.Source.Line, fact.Source.Column}, {fact.Binding.Line, fact.Binding.Column}, {fact.Iterable.Line, fact.Iterable.Column}} {
			offset := textPositionOffset(text, token.line, token.column)
			result, ok := hoverForSource(uri, text, offsetPosition(text, offset))
			if !ok || !strings.Contains(result.Contents.Value, fact.Display()) {
				t.Fatalf("hover at %+v: %+v; want %s", token, result, fact.Display())
			}
			if result.Range.Start != offsetPosition(text, offset) {
				t.Fatalf("wrong hover range: %+v", result.Range)
			}
		}
	}
	invalid, err := os.ReadFile("../../testdata/tooling/iterator_resolution_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text = string(invalid)
	offset := strings.Index(text, "for value")
	result, _ := hoverForSource(uri, text, offsetPosition(text, offset))
	if strings.Contains(result.Contents.Value, "Next target:") {
		t.Fatalf("invalid target advertised: %+v", result)
	}
}
