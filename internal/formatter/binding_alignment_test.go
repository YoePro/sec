package formatter

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Declaration tables preserve every syntax/literal/comment token, handle all
// configured indentation widths and styles, and reach a fixed point. Existing
// field/type/tag, register, parameter and named-type clients remain covered by
// their dedicated tests in the same formatter suite.
// Rules: rules/tooling/formatter.md — § 9(1)–(10), § 12(4)–(6), § 29, § 30.
func TestBindingAlignment(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/binding_alignment.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	expected, err := os.ReadFile("../../testdata/formatter/binding_alignment.expected.sec")
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
			lines := strings.Split(string(expected), "\n")
			for i, line := range lines {
				indent := leadingSpaces(line)
				// Structural indentation changes; alignment padding is in spaces.
				lines[i] = strings.Repeat(" ", indent/4*width+indent%4) + line[indent:]
			}
			want := strings.Join(lines, "\n")
			options := Options{IndentationWidth: width, VerticalStyle: style, MaxAlignmentPadding: 16}
			got := Format(Source{Text: string(input)}, options).Text
			if got != want {
				t.Fatalf("width %d style %s:\n%s\nwant:\n%s", width, style, got, want)
			}
			if again := Format(Source{Text: got}, options).Text; again != got {
				t.Fatalf("not idempotent: width %d style %s:\n%s", width, style, again)
			}
			if !reflect.DeepEqual(tokens(string(input)), tokens(got)) {
				t.Fatal("declaration alignment changed tokens")
			}
			p := parser.New(lexer.New(got))
			p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatal(p.Errors())
			}
		}
	}
}

// Padding fallback applies to the whole compatible group, independently for
// primary and secondary columns, with configured space limits (MD-098).
// Rules: rules/tooling/formatter.md — § 4(14), § 9(9)–(10), § 29(1).
func TestBindingAlignmentPaddingLimit(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/binding_alignment_padding.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/binding_alignment_padding.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	options := Options{MaxAlignmentPadding: 1}
	got := Format(Source{Text: string(input)}, options).Text
	if got != string(want) {
		t.Fatalf("padding fallback:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, options).Text; again != got {
		t.Fatal("padding fallback is not idempotent")
	}
}
