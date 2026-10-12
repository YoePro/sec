package formatter

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestDestructionSyntaxPreservation checks cleanup and transfer syntax, literal
// and comment opacity, and idempotence without removing even redundant discard.
// Rules: rules/tooling/formatter.md — §§3(3,12),19(7–9),21(2–9),31(8);
// rules/memory/destruction.md — §33.2(4),§34.
func TestDestructionSyntaxPreservation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/destruction_syntax.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/destruction_syntax.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []VerticalStyle{VerticalCompact, VerticalStructured} {
		options := Options{VerticalStyle: style}
		got := Format(Source{Text: string(input)}, options).Text
		if style == VerticalCompact && got != string(want) {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
		if again := Format(Source{Text: got}, options).Text; again != got {
			t.Fatalf("not idempotent:\n%s", again)
		}
		for _, text := range []string{string(input), got} {
			p := parser.New(lexer.New(text))
			p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
		}
		tokens := func(text string) []string {
			l := lexer.New(text)
			var result []string
			for token := l.NextToken(); token.Type != lexer.EOF; token = l.NextToken() {
				if token.Type != lexer.COMMA {
					result = append(result, string(token.Type)+":"+token.Lexeme)
				}
			}
			return result
		}
		if !reflect.DeepEqual(tokens(string(input)), tokens(got)) {
			t.Fatal("formatter changed cleanup, move, comment or literal tokens")
		}
	}
}
