package formatter

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// MD-049, formatter.md § 31A: decimal literals keep their exact spelling,
// including significant trailing zeros and digit separators, `scale` and
// `range` clauses keep their order, the formatted source parses to the same
// program, and a second pass changes nothing.
func TestFormatPreservesDecimalSpellingAndScaleClauses(t *testing.T) {
	input := "module main\n\ntype Money decimal   scale 2   range 0.00..100.50\ntype Rate decimal range 0.000..<1.0 scale  3\ntype Exact decimal scale -2\ntype Price Money default 1.200\n\nfn F() Money {\n    let a: Money := 1.50\n    let b := 0.000_100\n    let c := 2.50m\n    discard b\n    discard c\n    return a\n}\n"
	result := Format(Source{Text: input}, Options{})
	if result.Malformed {
		t.Fatalf("malformed:\n%s", result.Text)
	}
	for _, want := range []string{
		"type Money decimal scale 2 range 0.00..100.50",
		"decimal range 0.000..<1.0 scale 3",
		"type Exact decimal scale -2",
		"default 1.200",
		":= 1.50", ":= 0.000_100", ":= 2.50m",
	} {
		if !strings.Contains(result.Text, want) {
			t.Fatalf("missing %q in\n%s", want, result.Text)
		}
	}
	if again := Format(Source{Text: result.Text}, Options{}).Text; again != result.Text {
		t.Fatalf("second pass changed:\n%s", again)
	}
	parse := func(text string) string {
		p := parser.New(lexer.NewWithFile(text, "decimal.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatalf("parse: %v\n%s", p.Errors(), text)
		}
		return parsed.Program.String()
	}
	if before, after := parse(input), parse(result.Text); before != after {
		t.Fatalf("formatting changed the parse:\nbefore %s\nafter  %s", before, after)
	}
}
