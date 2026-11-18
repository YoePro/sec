package formatter

import (
	"fmt"
	"strings"
	"testing"
)

// TestIfConformanceFormatting preserves mandatory branch blocks, attaches
// else-if/else to the closing brace, and removes only redundant whole-condition
// parentheses when Language Corrections are requested. Grouping and the
// configured indentation width survive repeated formatting.
// Rules: rules/control-flow/flowcontrol_if.md — §§2,25;
// rules/tooling/formatter.md — §§17,27.
func TestIfConformanceFormatting(t *testing.T) {
	for _, width := range []int{2, 4, 6} {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(fmt.Sprintf("width%d-crlf%t", width, newline == "\r\n"), func(t *testing.T) {
				input := "fn Choose(a: bool, b: bool, c: bool) int {\nif (a) {\nreturn 1\n} else if ((a || b) && c) {\nreturn 2\n} else {\nreturn 3\n}\n}\n"
				indent := strings.Repeat(" ", width)
				want := "fn Choose(a: bool, b: bool, c: bool) int {\n" + indent + "if a {\n" + indent + indent + "return 1\n" + indent + "} else if (a || b) && c {\n" + indent + indent + "return 2\n" + indent + "} else {\n" + indent + indent + "return 3\n" + indent + "}\n}\n"
				input = strings.ReplaceAll(input, "\n", newline)
				options := Options{Fix: true, IndentationWidth: width}
				got := Format(Source{Text: input}, options).Text
				if got != want {
					t.Fatalf("got:\n%s\nwant:\n%s", got, want)
				}
				if again := Format(Source{Text: got}, options).Text; again != got {
					t.Fatalf("not idempotent:\n%s", again)
				}
			})
		}
	}
}
