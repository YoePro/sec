package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Completion/hover project the exact registry, and retired calls share the
// compiler's mandatory ID, source span and borrowed-view explanation.
// Rules: rules/compiler/compiler_known_members.md §21(2)-(4);
// rules/tooling/lsp.md — Completion, Hover, Shared diagnostic model.
func TestStringViewRegistryLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/string_ranges/registry_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, d := range analyze("file:///string-view-registry/main.sec", source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	changed := strings.Replace(source, "let count: uint := view.Len", "view.", 1)
	offset := strings.Index(changed, "view.") + len("view.")
	items := completeSource("", changed, offset)
	want := []string{"Len", "SizeOf", "IsEmpty", "GetRune", "RuneAt", "GetByte", "ByteAt", "CharAt", "StartsWith", "ToString"}
	assertCompletionLabels(t, items, want)
	for _, item := range items {
		if item.Label == "Slice" {
			t.Fatal("retired completion", item)
		}
	}
	// Check a real source-loaded string surface too, not only the intrinsic view.
	stringSource := strings.Replace(source, "return Ok(try text[a..<b])", "text.", 1)
	atString := strings.Index(stringSource, "text.") + len("text.")
	stringItems := completeSource("", stringSource, atString)
	if len(stringItems) == 0 {
		t.Fatal("missing string completions")
	}
	for _, item := range stringItems {
		if item.Label == "Slice" {
			t.Fatal("retired string completion", item)
		}
	}

	for _, name := range want {
		marker := "view." + name
		at := strings.Index(source, marker) + len("view.")
		hover, ok := hoverForSource("", source, offsetPosition(source, at))
		if !ok || !strings.Contains(hover.Contents.Value, "StringView."+name) {
			t.Fatal(name, hover, ok)
		}
	}
	invalid, err := os.ReadFile("../../testdata/sema/string_ranges/slice_removed_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze("file:///string-view-retirement/main.sec", string(invalid)) {
		if d.Severity != 1 {
			continue
		}
		count++
		if d.Code != diagnostics.UnknownCallTarget || !strings.Contains(d.Message, "borrowed StringView") || d.Range.End.Character-d.Range.Start.Character != len("Slice") {
			t.Fatal(d)
		}
	}
	if count != 5 {
		t.Fatal(count)
	}
}
