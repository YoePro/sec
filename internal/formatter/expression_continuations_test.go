package formatter

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Inline-start return expressions and if/else-if conditions use one configured level,
// preserve operators/comments and nested delimiter structure, and reach a fixed
// point without changing the token stream or following statements.
// Rules: rules/tooling/formatter.md — §4(8), §6(3), §9(15), §29.
func TestExpressionContinuationIndentation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/expression_continuations.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	expected, err := os.ReadFile("../../testdata/formatter/expression_continuations.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	tokens := func(text string) []string {
		var out []string
		lex := lexer.New(text)
		for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
			out = append(out, string(token.Type)+":"+token.Lexeme)
		}
		return out
	}
	for _, width := range []int{2, 4, 6} {
		for _, style := range []VerticalStyle{VerticalStructured, VerticalCompact} {
			wantLines := strings.Split(string(expected), "\n")
			for i, line := range wantLines {
				indent := leadingSpaces(line)
				wantLines[i] = strings.Repeat(" ", indent/4*width) + line[indent:]
			}
			want := strings.Join(wantLines, "\n")
			options := Options{IndentationWidth: width, VerticalStyle: style}
			got := Format(Source{Text: string(input)}, options).Text
			if got != want {
				t.Fatalf("width %d style %s:\n%s\nwant:\n%s", width, style, got, want)
			}
			if again := Format(Source{Text: got}, options).Text; again != got {
				t.Fatal("return layout is not idempotent", width, style)
			}
			if !reflect.DeepEqual(tokens(string(input)), tokens(got)) {
				t.Fatal("return formatting changed tokens")
			}
			p := parser.New(lexer.New(got))
			p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
		}
	}
}

// Multiline literal contents are program data, including a closing literal
// line followed by an operator; only the subsequent operand receives indentation.
// Rules: rules/tooling/formatter.md — §6(3), §6(8), §29.
func TestReturnContinuationLiteralPreservation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/return_continuation_literals.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/return_continuation_literals.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertFormat(t, string(input), string(want))
}
