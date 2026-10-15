package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LSP formatting and sec fmt share the exact configured return/if continuation
// layout, including all supported widths and a stable second formatting pass.
// Rules: rules/tooling/formatter.md — §4(8), §6(3), §28; rules/tooling/lsp.md — "Shared formatter".
func TestLSPExpressionContinuation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/expression_continuations.sec")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("../../testdata/formatter/expression_continuations.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(fmt.Sprintf("[format]\nindentation_width = %d\n", width)), 0644); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "main.sec")
		lines := strings.Split(string(expected), "\n")
		for i, line := range lines {
			n := len(line) - len(strings.TrimLeft(line, " "))
			lines[i] = strings.Repeat(" ", n/4*width) + line[n:]
		}
		got, err := formatSourceForDocument(uriFromPath(file), string(input))
		if err != nil || got != strings.Join(lines, "\n") {
			t.Fatal("LSP return indentation differs", width, err, got)
		}
		again, err := formatSourceForDocument(uriFromPath(file), got)
		if err != nil || again != got {
			t.Fatal("LSP second pass changed return indentation", err)
		}
	}
}
