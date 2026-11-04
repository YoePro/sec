package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Both styles preserve parser meaning, documentation attachment, ordinary
// alignment-group boundaries and literal bytes while rendering structural
// separation. All supported widths, CRLF and correction modes reach a fixed point.
// Rules: rules/tooling/formatter.md — §4(6–8),§7(1–13),§12,§25,§29–30.
func TestVerticalSpacing(t *testing.T) {
	source, err := os.ReadFile("../../testdata/formatter/vertical_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	before := verticalDocumentation(t, string(source))
	tokens := func(text string) []string {
		var result []string
		lex := lexer.New(text)
		for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
			if token.Type != lexer.COMMA {
				result = append(result, string(token.Type)+":"+token.Lexeme)
			}
		}
		return result
	}
	for _, style := range []VerticalStyle{VerticalStructured, VerticalCompact} {
		name := "vertical_spacing.expected.sec"
		if style == VerticalCompact {
			name = "vertical_spacing.compact.expected.sec"
		}
		golden, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{2, 4, 6} {
			for _, crlf := range []bool{false, true} {
				for _, fix := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%d/crlf%t/fix%t", style, width, crlf, fix), func(t *testing.T) {
						text := string(source)
						if crlf {
							text = strings.ReplaceAll(text, "\n", "\r\n")
						}
						options := Options{VerticalStyle: style, IndentationWidth: width, Fix: fix}
						want := reindentOwnershipGolden(string(golden), width)
						// Literal continuation indentation is data, not layout.
						want = strings.ReplaceAll(want, "\nlast`", "\n  last`")
						got := Format(Source{Text: text}, options)
						if got.Malformed || got.Text != want {
							t.Fatalf("vertical spacing:\n%s\nwant:\n%s", got.Text, want)
						}
						if after := verticalDocumentation(t, got.Text); !reflect.DeepEqual(before, after) {
							t.Fatalf("documentation attachment changed: %v -> %v", before, after)
						}
						if !reflect.DeepEqual(tokens(string(source)), tokens(got.Text)) {
							t.Fatal("vertical style changed syntax, comment or literal tokens")
						}
						if again := Format(Source{Text: got.Text}, options); again.Text != got.Text || again.Malformed {
							t.Fatalf("not idempotent:\n%s", again.Text)
						}
					})
				}
			}
		}
	}
}

// Project documentation through its exact parser-established target identity;
// a detached comment must never become documentation when compacting gaps.
// Rules: rules/foundations/lexical_structure.md — §5.4;
// rules/tooling/formatter.md — §7(7–9),§12,§29(4).
func verticalDocumentation(t *testing.T, text string) map[string]string {
	t.Helper()
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	result := map[string]string{}
	for _, attachment := range program.Documentation {
		name := fmt.Sprintf("%T", attachment.Declaration)
		if function, ok := attachment.Declaration.(*ast.FunctionDeclaration); ok {
			name += ":" + function.Name.Value
		}
		for _, comment := range attachment.Comments {
			result[comment.Lexeme] = name
		}
	}
	if _, attached := result["/** Deliberately detached documentation. */"]; attached {
		t.Fatal("detached documentation acquired a target")
	}
	return result
}
