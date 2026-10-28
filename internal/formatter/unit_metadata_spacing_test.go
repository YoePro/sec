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

// Metadata spacing changes trivia and canonical key case only: exact numeric
// tokens, concrete enum identities, legacy values and unit suffixes survive.
// Rules: rules/tooling/formatter.md §§6,22(11–13),29; MD-062 §§5(4),12(10),13(8).
func TestUnitMetadataCanonicalSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	before := metadataTokenValues(t, string(input))
	for _, width := range []int{2, 4, 6} {
		for _, fix := range []bool{false, true} {
			for _, crlf := range []bool{false, true} {
				t.Run(fmt.Sprintf("indent%d/fix%t/crlf%t", width, fix, crlf), func(t *testing.T) {
					source := string(input)
					if crlf {
						source = strings.ReplaceAll(source, "\n", "\r\n")
					}
					options := Options{IndentationWidth: width, Fix: fix}
					got := Format(Source{Text: source}, options)
					want := reindentOwnershipGolden(string(golden), width)
					if got.Malformed || got.Text != want {
						t.Fatalf("got malformed=%t:\n%s\nwant:\n%s", got.Malformed, got.Text, want)
					}
					if !reflect.DeepEqual(before, metadataTokenValues(t, got.Text)) {
						t.Fatalf("metadata tokens changed:\n%s", got.Text)
					}
					if again := Format(Source{Text: got.Text}, options); again.Malformed || again.Text != got.Text {
						t.Fatalf("second pass changed source:\n%s", again.Text)
					}
				})
			}
		}
	}
}

// Repeated unary signs must not fuse into ++/--; a numeric dot is never an
// enum separator, comments/literals are preserved, and impl <U> need not have
// its unit declaration in the same source file to own metadata spacing.
// Rules: rules/foundations/grammar.md — "Unit metadata inside impl";
// rules/tooling/formatter.md §§22(11–13),25,29(2–4).
func TestUnitMetadataSpacingTokenBoundaries(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"- -1", "- -1"}, {"+ +1", "+ +1"}, {"1 . 5", "1 . 5"},
		{"1+- 2/(- 3)", "1 + -2 / (-3)"},
		{"2^- 3", "2 ^ -3"}, {"1/* exact */ /60", "1/* exact */ / 60"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			source := "impl <external> {\nScale:" + tc.value + "\n}\n"
			got := Format(Source{Text: source}, Options{}).Text
			if !strings.Contains(got, "Scale: "+tc.want) {
				t.Fatalf("wrong spacing:\n%s", got)
			}
			if !reflect.DeepEqual(metadataTokenValues(t, source), metadataTokenValues(t, got)) {
				t.Fatalf("tokens changed:\n%s", got)
			}
			if again := Format(Source{Text: got}, Options{}).Text; again != got {
				t.Fatalf("not idempotent:\n%s", again)
			}
		})
	}
	broken := "impl <external> {\nScale: (1 /\n"
	if got := Format(Source{Text: broken}, Options{}); !got.Malformed || got.Text != broken {
		t.Fatalf("changed malformed source: %+v", got)
	}
}

// metadataTokenValues compares the parser-owned values byte-for-byte at token
// granularity. Only the approved key-case/underscore migration is normalized.
// Rules: rules/types/units.md — "Formatter requirements"; MD-062 §12(10).
func metadataTokenValues(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors %v:\n%s", p.Errors(), source)
	}
	var values []string
	ast.Visit(program, func(node any) bool {
		metadata, ok := node.(*ast.UnitMetadataDeclaration)
		if !ok {
			return true
		}
		values = append(values, "key:"+strings.ReplaceAll(strings.ToLower(metadata.Name), "_", ""))
		for _, token := range metadata.Value {
			values = append(values, string(token.Type)+":"+token.Lexeme)
		}
		return true
	})
	return values
}
