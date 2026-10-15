package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// sec fmt uses the project's indentation width for return/if continuations and
// its check mode accepts the resulting canonical file without further edits.
// Rules: rules/tooling/formatter.md — §4(8), §6(3), §28.
func TestFmtExpressionContinuation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/expression_continuations.sec")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("../../testdata/formatter/expression_continuations.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		_, file := writeFmtProject(t, fmt.Sprintf("[format]\nindentation_width = %d\n", width), string(input))
		if err := runFmtCommand([]string{file}); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(expected), "\n")
		for i, line := range lines {
			n := len(line) - len(strings.TrimLeft(line, " "))
			lines[i] = strings.Repeat(" ", n/4*width) + line[n:]
		}
		if got := readFile(t, file); got != strings.Join(lines, "\n") {
			t.Fatal("CLI return indentation differs", width, got)
		}
		if err := runFmtCommand([]string{"--check", file}); err != nil {
			t.Fatal(err)
		}
	}
}
