package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestFmtTryHandlerBlocks keeps CLI output and check mode on the shared
// BUG-000030 golden under both project vertical styles and configured indents.
// Rules: rules/tooling/formatter.md — §§4,19,28–29.
func TestFmtTryHandlerBlocks(t *testing.T) {
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
			config := fmt.Sprintf("[format]\nindentation_width = %d\nvertical_style = %q\n", indent, style)
			_, file := writeFmtProject(t, config, string(input))
			if err := runFmtCommand([]string{file}); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, file); got != want {
				t.Fatalf("indent %d %s differs from golden:\n%s", indent, style, got)
			}
			if err := runFmtCommand([]string{"--check", file}); err != nil {
				t.Fatal("CLI rejected its canonical handler layout", err)
			}
		}
	}
}
