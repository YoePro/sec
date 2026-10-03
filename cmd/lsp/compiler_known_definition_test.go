package main

import (
	"strings"
	"testing"
)

func TestDefinitionOfCompilerKnownMemberNavigatesToSyntheticDefinition(t *testing.T) {
	source := "module main\n\nfn Fill(values: ref mut list[int]) Result[void, CollectionError] {\n    try values.Append(1)\n    discard values.RemoveAt(0)\n    return Ok()\n}\n"
	offset := strings.Index(source, "Append")
	pos := offsetPosition(source, offset)
	locations := definitionsForSource("file:///tmp/synthetic.sec", source, pos)
	if len(locations) != 1 || locations[0].URI != "sec-compiler-known:/CKM-LIST-APPEND.sec" {
		t.Fatalf("locations = %#v", locations)
	}
	text, ok := compilerKnownDefinitionText(locations[0].URI)
	if !ok {
		t.Fatal("no synthetic text")
	}
	for _, want := range []string{
		"Synthetic read-only definition",
		"not source code",
		"Member:     Append (CKM-LIST-APPEND)",
		"Rule:       rules/collections/collections.md — 13.5 Core list methods",
		"Receiver:   list[T] (mutable)",
		"Effects:    none",
		"Unsafe:     no",
		"Target:     none; available on every target plan",
		"Structural: changes the receiver's element count or storage",
		"fn Append(value: T) Result[void, CollectionError]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("synthetic text lacks %q:\n%s", want, text)
		}
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if got := lines[locations[0].Range.Start.Line]; got != "fn Append(value: T) Result[void, CollectionError]" {
		t.Fatalf("location line = %q", got)
	}
}

func TestSyntheticDefinitionTextRendersFromIDAlone(t *testing.T) {
	text, ok := compilerKnownDefinitionText("sec-compiler-known:/CKM-DYNAMIC-ARRAY-REMOVEAT.sec")
	if !ok || !strings.Contains(text, "fn RemoveAt(index: uint) Option[T]") || !strings.Contains(text, "6.9 `RemoveAt`") {
		t.Fatalf("text = %q", text)
	}
	if _, ok := compilerKnownDefinitionText("file:///tmp/x.sec"); ok {
		t.Fatal("ordinary URIs are not synthetic definitions")
	}
}

func TestCompilerKnownMemberHoverShowsRegistrySignature(t *testing.T) {
	source := "module main\n\nfn Drop(values: ref mut list[int]) void {\n    discard values.RemoveAt(0)\n}\n"
	offset := strings.Index(source, "RemoveAt")
	hover, ok := hoverForSource("file:///tmp/hover.sec", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "fn RemoveAt(index: uint) Option[T]") {
		t.Fatalf("hover = %#v", hover)
	}
}
