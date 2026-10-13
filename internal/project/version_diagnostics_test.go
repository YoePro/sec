package project

import (
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§8–11,48(1–3).
func TestVersionSnapshotDiagnostics(t *testing.T) {
	fixture := manifestFixture(t)
	if items := ValidateVersionSnapshot(fixture); len(items) != 0 {
		t.Fatal(items)
	}
	for _, tc := range []struct{ old, new, want string }{
		{"build = 123", "  build = -1  # bad baseline", "negative"},
		{"build = 123", "build = 9223372036854775808", "out of range"},
		{"[major].[minor].[revision]-[userdefined].[build]", "[major].[missing]", "unknown"},
		{"build = 123", "build = 123\nbuild = 124", "already"},
		{"userdefined = \"beta\"", "userdefined = ''", "target lsp version"},
	} {
		text := strings.Replace(fixture, tc.old, tc.new, 1)
		items := ValidateVersionSnapshot(text)
		if len(items) == 0 {
			t.Fatal(tc, text)
		}
		for _, item := range items {
			if item.Line < 1 || item.Column < 1 || item.EndColumn < item.Column || item.Message == "" {
				t.Fatal(item)
			}
		}
		// Target inheritance must point at its owning override, not file line 1.
		if tc.want == "target lsp version" && items[0].Line != manifestKeyLine(text, []string{"target", "lsp", "version"}) {
			t.Fatal(items)
		}
	}
	if items := ValidateVersionSnapshot(""); len(items) != 1 || items[0].Line != 1 {
		t.Fatal(items)
	}
	text := "[version]\nmajor=1\nminor=0\nrevision=0\n  build = -1 # comment\nformat='[build]'\n"
	items := ValidateVersionSnapshot(text)
	if len(items) != 1 || items[0].Line != 5 || items[0].Column != 3 || items[0].EndColumn != 13 {
		t.Fatal(items)
	}
	if items := ValidateVersionSnapshot(strings.Replace(text, "[version]", "[[version]]", 1)); len(items) != 1 || !strings.Contains(items[0].Message, "TOML table") {
		t.Fatal(items)
	}
	text = strings.Replace(text, "build = -1", "build = 1", 1) + "[target.worker]\nversion=5\n"
	if items := ValidateVersionSnapshot(text); len(items) != 1 || !strings.Contains(items[0].Message, "target.worker.version must be a TOML table") {
		t.Fatal(items)
	}
}
