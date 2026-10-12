package formatter

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCSTDelimiterSpacing checks nested aggregates, opaque literals, comments,
// foreign qualifiers and assertion punctuation, plus the second-pass fixed
// point and preservation of all non-comma source tokens.
// Rules: rules/tooling/formatter.md — §5,§8(6),§11,§12,§23(1),§29.
func TestCSTDelimiterSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/cst_delimiter_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/cst_delimiter_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(input), string(want)} {
		got := Format(Source{Text: text}, Options{}).Text
		if got != string(want) {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
		p := parser.New(lexer.New(got))
		p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
	}
	tokens := func(text string) []string {
		l := lexer.New(text)
		result := []string{}
		for token := l.NextToken(); token.Type != lexer.EOF; token = l.NextToken() {
			if token.Type != lexer.COMMA {
				result = append(result, string(token.Type)+":"+token.Lexeme)
			}
		}
		return result
	}
	if !reflect.DeepEqual(tokens(string(input)), tokens(string(want))) {
		t.Fatal("formatting changed non-comma source tokens")
	}
}
