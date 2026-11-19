package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryHandlerBlockLayout verifies BUG-000030 across configured indentation,
// styles and widths, including continuation after thread configuration, nested
// arm blocks and following statements/declarations. Tokens never change.
// Rules: rules/tooling/formatter.md — §§4,6,8,19(2–6),29.
func TestTryHandlerBlockLayout(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/try_handler_blocks.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/try_handler_blocks.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	tokens := func(text string) []string {
		var result []string
		lex := lexer.New(text)
		for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
			result = append(result, string(token.Type)+":"+token.Lexeme)
		}
		return result
	}
	for _, width := range []int{2, 4, 6} {
		for _, style := range []VerticalStyle{VerticalStructured, VerticalCompact} {
			for _, lineWidth := range []uint64{80, 120, 200} {
				t.Run(fmt.Sprintf("indent%d/%s/width%d", width, style, lineWidth), func(t *testing.T) {
					lines := strings.Split(string(golden), "\n")
					for i, line := range lines {
						indent := leadingSpaces(line)
						lines[i] = strings.Repeat(" ", indent/4*width) + line[indent:]
					}
					want := strings.Join(lines, "\n")
					options := Options{IndentationWidth: width, VerticalStyle: style, LineWidth: lineWidth}
					got := Format(Source{Text: string(input)}, options)
					if got.Malformed || got.Text != want {
						t.Fatalf("malformed=%v\ngot:\n%s\nwant:\n%s", got.Malformed, got.Text, want)
					}
					if again := Format(Source{Text: got.Text}, options); again.Malformed || again.Text != want {
						t.Fatal("handler formatting changed on second pass", again.Text)
					}
					p := parser.New(lexer.New(got.Text))
					p.ParseProgram()
					if len(p.Errors()) != 0 || !reflect.DeepEqual(tokens(string(input)), tokens(got.Text)) {
						t.Fatal("formatting changed syntax", p.Errors())
					}
				})
			}
		}
	}
}
