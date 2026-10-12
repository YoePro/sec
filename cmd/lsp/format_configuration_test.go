package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LSP formatting resolves the same [format] configuration as sec fmt: the
// document's project settings, the defaults for a stand-alone file, and an
// error with no edit for an invalid configuration.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(15), § 4(22)
//   - rules/tooling/lsp.md — "Shared formatter"
func TestLSPFormattingUsesProjectFormatConfiguration(t *testing.T) {
	text := "module main\n\nfn F() int {\nreturn 1\n}\n"
	project := func(manifest string) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".sec"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return uriFromPath(filepath.Join(root, "main.sec"))
	}
	formatted, err := formatSourceForDocument(project("[format]\nindentation_width = 2\n"), text)
	if err != nil || !strings.Contains(formatted, "\n  return 1\n") {
		t.Fatalf("project formatting = %q, %v", formatted, err)
	}
	formatted, err = formatSourceForDocument(uriFromPath(filepath.Join(t.TempDir(), "alone.sec")), text)
	if err != nil || !strings.Contains(formatted, "\n    return 1\n") {
		t.Fatalf("stand-alone formatting = %q, %v", formatted, err)
	}
	if _, err := formatSourceForDocument(project("[format]\nformat_version = 2\n"), text); err == nil {
		t.Fatal("unsupported format_version produced an edit")
	}
}
