package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A parameter or local is presented from the binding Sema resolved at the
// hovered position. A same-named parameter of another function in a sibling
// module file must not supply the type, as it did through the module-wide
// name table (sec/stdlib/str: Fields(value: string) hovered as rune because
// builder.sec declares the overloaded method AppendRepeat(value: rune, ...)).
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
func TestHoverPresentsTheBindingAtThePositionNotASameNamedSibling(t *testing.T) {
	dir := t.TempDir()
	sibling := "module text\n\ntype Builder struct {\n    size: uint,\n}\n\nimpl Builder {\n    fn Put(value: string) uint {\n        return 0\n    }\n    fn Put(value: rune) uint {\n        return 1\n    }\n}\n"
	source := "module text\n\nfn Count(value: string) int {\n    let mut count := 0\n    for character in value {\n        discard character\n        count += 1\n    }\n    return count\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "z.sec"), []byte(sibling), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "b.sec")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		needle string
		delta  int
		want   string
	}{
		{"Count(value", len("Count("), "value: string"},
		{"in value", len("in "), "value: string"},
		{"for character", len("for "), "character: rune"},
	}
	for _, test := range cases {
		offset := strings.Index(source, test.needle) + test.delta
		hover, ok := hoverForSource("file://"+path, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "```sec\n"+test.want+"\n```") {
			t.Errorf("hover at %q = %v %q, want %q", test.needle, ok, hover.Contents.Value, test.want)
		}
	}
}

// Completion offers the visible local or parameter binding of the completed
// file with its own type. A same-named parameter of a sibling-file method
// previously replaced it in the module-wide name table, so the parameter was
// not offered at all.
//
// Rules:
//   - rules/tooling/lsp.md — "Completion"
func TestCompletionOffersTheVisibleBindingNotASameNamedSibling(t *testing.T) {
	dir := t.TempDir()
	sibling := "module text\n\ntype Builder struct {\n    size: uint,\n}\n\nimpl Builder {\n    fn Put(value: string) uint {\n        return 0\n    }\n    fn Put(value: rune) uint {\n        return 1\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "z.sec"), []byte(sibling), 0o644); err != nil {
		t.Fatal(err)
	}
	source := "module text\n\nfn Count(value: string) int {\n    let n := va|\n    return 0\n}\n\nfn Other(value: bool) bool {\n    return value\n}\n"
	offset := strings.Index(source, "|")
	text := strings.Replace(source, "|", "", 1)
	path := filepath.Join(dir, "b.sec")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var offered []string
	for _, item := range completeSource("file://"+path, text, offset) {
		if item.Label == "value" {
			offered = append(offered, item.Detail)
		}
	}
	if len(offered) != 1 || offered[0] != "string" {
		t.Fatalf("value completions = %q, want one with detail string", offered)
	}
}
