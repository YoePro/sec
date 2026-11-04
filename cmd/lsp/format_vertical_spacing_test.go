package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The editor preserves authored declaration gaps with either project style.
// Rules: rules/tooling/formatter.md — §7(4),§28.
func TestLSPDeclarationBlankLines(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/declaration_blank_lines.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		root := t.TempDir()
		writeWorkspaceSource(t, root, ".sec/sec.toml", "[format]\nvertical_style = \""+style+"\"\n")
		uri := uriFromPath(writeWorkspaceSource(t, root, "source.sec", string(input)))
		got, err := formatSourceForDocument(uri, string(input))
		if err != nil || got != string(input) {
			t.Fatalf("%s removed authored declaration spacing: %v\n%s", style, err, got)
		}
		if again, err := formatSourceForDocument(uri, got); err != nil || again != got {
			t.Fatal("second pass removed declaration spacing", err)
		}
	}
}

// LSP document formatting uses the same project-owned structural separation as
// sec fmt, including detached documentation and preserved raw-string contents.
// Rules: rules/tooling/formatter.md — §4(6–7,22),§7(3–13),§28;
// rules/tooling/lsp.md — Shared formatter.
func TestLSPVerticalSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/vertical_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".sec"), 0700); err != nil {
			t.Fatal(err)
		}
		manifest := "[format]\nvertical_style = \"" + style + "\"\n"
		if err := os.WriteFile(filepath.Join(root, ".sec/sec.toml"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		name := "vertical_spacing.expected.sec"
		if style == "compact" {
			name = "vertical_spacing.compact.expected.sec"
		}
		want, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(root, "source.sec"))
		got, err := formatSourceForDocument(uri, string(input))
		if err != nil || got != string(want) {
			t.Fatalf("%s LSP differs from shared golden: %v\n%s", style, err, got)
		}
		if again, err := formatSourceForDocument(uri, got); err != nil || again != got {
			t.Fatal("LSP second pass changed vertical spacing", err)
		}
	}
}

// Both editor styles retain the same mandatory blank line as sec fmt.
// Rules: rules/tooling/formatter.md — §7(7a),§28; lsp.md — Shared formatter.
func TestLSPFunctionCommentSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		root := t.TempDir()
		writeWorkspaceSource(t, root, ".sec/sec.toml", "[format]\nvertical_style = \""+style+"\"\n")
		uri := uriFromPath(writeWorkspaceSource(t, root, "source.sec", string(input)))
		got, err := formatSourceForDocument(uri, string(input))
		if err != nil || got != string(want) {
			t.Fatalf("%s LSP removed required comment separation: %v\n%s", style, err, got)
		}
		if again, err := formatSourceForDocument(uri, got); err != nil || again != got {
			t.Fatal("second pass changed required separation", err)
		}
	}
}
