package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestLSPTryHandlerBlocks projects exactly the shared CLI/formatter golden for
// BUG-000030 and remains stable on the next document-formatting request.
// Rules: rules/tooling/formatter.md — §§4,19,28–29;
// rules/tooling/lsp.md — "Formatting".
func TestLSPTryHandlerBlocks(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/try_handler_blocks.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/try_handler_blocks.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, indent := range []int{2, 4, 6} {
		for _, style := range []string{"structured", "compact"} {
			lines := strings.Split(string(golden), "\n")
			for i, line := range lines {
				spaces := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = strings.Repeat(" ", spaces/4*indent) + line[spaces:]
			}
			want := strings.Join(lines, "\n")
			root := t.TempDir()
			config := fmt.Sprintf("[format]\nindentation_width = %d\nvertical_style = %q\n", indent, style)
			writeWorkspaceSource(t, root, ".sec/sec.toml", config)
			uri := uriFromPath(writeWorkspaceSource(t, root, "handlers.sec", string(input)))
			got, err := formatSourceForDocument(uri, string(input))
			if err != nil || got != want {
				t.Fatalf("indent %d %s differs from golden: %v\n%s", indent, style, err, got)
			}
			if again, err := formatSourceForDocument(uri, got); err != nil || again != want {
				t.Fatal("LSP handler layout changed on the second request", err)
			}
		}
	}
}
