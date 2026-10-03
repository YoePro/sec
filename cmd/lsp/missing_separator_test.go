package main

import "testing"

func TestMissingSeparatorQuickFixInsertsCommaAfterPreviousItem(t *testing.T) {
	source := "module main\n\ntype Point struct {\n    x: int // first\n    y: int,\n}\n\nfn main() void {\n    let values := [\n        1\n        2,\n    ]\n}\n"
	reported := []diagnostic{}
	for _, item := range analyze("file:///tmp/missing_separator.sec", source) {
		if item.Code == "P2002" {
			reported = append(reported, item)
		}
	}
	if len(reported) != 2 {
		t.Fatalf("P2002 diagnostics = %#v", reported)
	}
	actions := missingSeparatorCodeActions("file:///tmp/missing_separator.sec", source, reported)
	if len(actions) != 2 {
		t.Fatalf("actions = %#v", actions)
	}
	text := source
	for _, action := range actions {
		if action.Title != "Insert missing ','" || action.Kind != "quickfix" || len(action.Diagnostics) != 1 {
			t.Fatalf("action = %#v", action)
		}
	}
	edits := []textEdit{}
	for _, action := range actions {
		edits = append(edits, action.Edit.Changes["file:///tmp/missing_separator.sec"]...)
	}
	text = applyTextEdits(text, edits)
	want := "module main\n\ntype Point struct {\n    x: int, // first\n    y: int,\n}\n\nfn main() void {\n    let values := [\n        1,\n        2,\n    ]\n}\n"
	if text != want {
		t.Fatalf("fixed text =\n%s", text)
	}
	for _, item := range analyze("file:///tmp/missing_separator.sec", text) {
		if item.Code == "P2002" {
			t.Fatalf("fixed source still reports %#v", item)
		}
	}
}

func TestMissingSeparatorQuickFixIgnoresUnprovenSameLineAdjacency(t *testing.T) {
	source := "module main\n\ntype Pair struct {\n    left: int right: int\n}\n"
	reported := analyze("file:///tmp/same_line.sec", source)
	if actions := missingSeparatorCodeActions("file:///tmp/same_line.sec", source, reported); len(actions) != 0 {
		t.Fatalf("same-line adjacency must not get a quick fix: %#v", actions)
	}
}
