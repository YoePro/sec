package main

import (
	"os"
	"strings"
	"testing"
)

// TestForBindingHeadersLSP presents compiler-validated modes at binding names,
// reference keywords and the for keyword, using the same formatting as CLI.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10;
// rules/tooling/lsp.md — Hover; rules/tooling/formatter.md — §28.
func TestForBindingHeadersLSP(t *testing.T) {
	assertFormattingGoldenLSP(t, "for_binding_headers")
	data, err := os.ReadFile("../../testdata/formatter/for_binding_headers.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///for-binding-modes/main.sec"
	a := analyzeNavigationSource(uri, source)
	count := 0
	for _, fact := range a.ForBindingModes() {
		if fact.Binding.File != pathFromURI(uri) {
			continue
		}
		count++
		for _, token := range []struct{ line, column int }{{fact.Binding.Line, fact.Binding.Column}, {fact.Loop.Line, fact.Loop.Column}, {fact.ModeToken.Line, fact.ModeToken.Column}} {
			if token.line == 0 {
				continue
			}
			offset := textPositionOffset(source, token.line, token.column)
			hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
			if !ok || !strings.Contains(hover.Contents.Value, fact.Display()) {
				t.Fatal(fact, hover, ok)
			}
		}
	}
	if count != 7 {
		t.Fatal(count)
	}
}
