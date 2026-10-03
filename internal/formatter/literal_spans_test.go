package formatter

import (
	"os"
	"testing"
)

// Rules:
//   - rules/foundations/lexical_structure.md — §14.3 "Interpolated strings"
//   - rules/tooling/formatter.md — § 29 literal contents are never rewritten
func TestFormatLeavesNestedInterpolationLiteralsAndIndentationIntact(t *testing.T) {
	source, err := os.ReadFile("../../testdata/lexer/interpolation_balanced_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(Source{Text: string(source)}, Options{}).Text; got != string(source) {
		t.Fatalf("canonical interpolation fixture changed:\n%s", got)
	}
}

func TestFormatIndentsAfterLiteralClosingBraces(t *testing.T) {
	input := "fn main() void {\nlet quoted := $\"{Format(\"text }\")}\"\nlet raw := `}`\nlet character := '}'\nlet following := 42\n}\n"
	want := "fn main() void {\n    let quoted := $\"{Format(\"text }\")}\"\n    let raw := `}`\n    let character := '}'\n    let following := 42\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
}

func TestLiteralEndsUsesLexerBoundaries(t *testing.T) {
	text := `call($"{Format("a }")}", 'x', ` + "`raw`" + `, "open`
	ends := literalEnds(text)
	want := map[int]int{5: 23, 25: 28, 30: 35, 37: len(text)}
	for start, end := range want {
		if ends[start] != end {
			t.Fatalf("literalEnds()[%d] = %d, want %d (all: %v)", start, ends[start], end, ends)
		}
	}
	if literalClosed(text[37:]) {
		t.Fatal("unterminated literal reported closed")
	}
}
