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

// TestFormatRawPointerOperations preserves ordinary and volatile method identity,
// unsafe boundaries and arithmetic operands across all indentation widths.
// Rules: rules/memory/raw_pointers.md §§5.2,11–14,36(1–4);
// rules/tooling/formatter.md §§17,19(11–14),29(2).
func TestFormatRawPointerOperations(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/raw_pointer_operations.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/raw_pointer_operations.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	before := rawPointerTokens(t, string(input))
	for _, width := range []int{2, 4, 6} {
		for _, crlf := range []bool{false, true} {
			for _, fix := range []bool{false, true} {
				t.Run(fmt.Sprintf("indent%d/crlf%t/fix%t", width, crlf, fix), func(t *testing.T) {
					source := string(input)
					if crlf {
						source = strings.ReplaceAll(source, "\n", "\r\n")
					}
					options := Options{IndentationWidth: width, Fix: fix}
					got := Format(Source{Text: source}, options)
					want := reindentOwnershipGolden(string(golden), width)
					if got.Malformed || got.Text != want {
						t.Fatalf("formatted (malformed=%t):\n%s\nwant:\n%s", got.Malformed, got.Text, want)
					}
					if after := rawPointerTokens(t, got.Text); !reflect.DeepEqual(before, after) {
						t.Fatalf("changed raw-pointer operations:\nbefore %v\nafter %v", before, after)
					}
					if again := Format(Source{Text: got.Text}, options); again.Malformed || again.Text != got.Text {
						t.Fatalf("unstable second pass:\n%s", again.Text)
					}
				})
			}
		}
	}
}

// rawPointerTokens compares exact non-layout tokens after validating grammar;
// optional list commas alone are omitted from the preservation comparison.
// Rules: rules/memory/raw_pointers.md §36; rules/tooling/formatter.md §29(2).
func rawPointerTokens(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("invalid source: %v\n%s", p.Errors(), source)
	}
	scanner := lexer.New(source)
	var tokens []string
	for token := scanner.NextToken(); token.Type != lexer.EOF; token = scanner.NextToken() {
		if token.Type != lexer.COMMA {
			tokens = append(tokens, string(token.Type)+":"+token.Lexeme)
		}
	}
	return tokens
}
