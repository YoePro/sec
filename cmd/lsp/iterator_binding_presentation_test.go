package main

import (
	"os"
	"strings"
	"testing"
)

// TestIteratorBindingPresentation projects exact resolved iterator identities
// into completion and use-site hover, including generic and borrowed views.
// Rules: rules/control-flow/flowcontrol_for.md — §§37–37.1;
// rules/compiler/semantic_ir.md — §109(1–4),§115(2); rules/tooling/lsp.md — §4.1(5–7).
func TestIteratorBindingPresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/iterator_binding_presentation.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///iterator-binding/main.sec"
	a := analyzeNavigationSource(uri, text)
	facts := a.IteratorResolutions()
	local := facts[:0]
	for _, fact := range facts {
		if fact.Source.File == pathFromURI(uri) {
			local = append(local, fact)
		}
	}
	if len(local) != 5 {
		t.Fatalf("missing valid iterator facts: %+v", local)
	}
	for _, fact := range local {
		start := textPositionOffset(text, fact.Binding.Line, fact.Binding.Column) + len("value")
		use := strings.Index(text[start:], "value") + start
		if use < start {
			t.Fatal("missing binding use")
		}
		items := completeSource(uri, text, use+3)
		found := false
		for _, item := range items {
			if item.Label == "value" {
				found = true
				if !strings.Contains(item.Documentation, fact.Display()) || item.Detail != fact.ElementType {
					t.Fatalf("completion lost resolution: %+v, want %s", item, fact.Display())
				}
			}
		}
		if !found {
			t.Fatal("missing value completion", fact)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, use))
		if !ok || !strings.Contains(hover.Contents.Value, fact.Display()) {
			t.Fatalf("hover lost resolution: %+v, want %s", hover, fact.Display())
		}
	}
	plain := strings.Index(text, "discard value\n") + len("discard ")
	plainHover, _ := hoverForSource(uri, text, offsetPosition(text, plain))
	if strings.Contains(plainHover.Contents.Value, "Next target:") {
		t.Fatal("iterator hover leaked into plain binding", plainHover)
	}
	for _, item := range completeSource(uri, text, plain+3) {
		if item.Label == "value" && strings.Contains(item.Documentation, "Next target:") {
			t.Fatalf("iterator facts leaked into plain binding: %+v", item)
		}
	}
}

// TestIteratorBindingInvalidPresentation never supplies a Next promise for
// duck typing or a shared interface view lacking mutable receiver authority.
// Rules: rules/control-flow/flowcontrol_for.md — §§37–37.1;
// rules/compiler/semantic_ir.md — §109(2),§115(2).
func TestIteratorBindingInvalidPresentation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/iterator_binding_presentation_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///iterator-invalid/main.sec"
	for start := 0; start < len(text); {
		relative := strings.Index(text[start:], "discard value")
		if relative < 0 {
			break
		}
		use := start + relative + len("discard ")
		for _, item := range completeSource(uri, text, use+3) {
			if item.Label == "value" && strings.Contains(item.Documentation, "Next target:") {
				t.Fatal(item)
			}
		}
		hover, _ := hoverForSource(uri, text, offsetPosition(text, use))
		if strings.Contains(hover.Contents.Value, "Next target:") {
			t.Fatal(hover)
		}
		start = use + len("value")
	}
}
