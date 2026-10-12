package formatter

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestFormatCompleteHeaderSpacing completes canonical declaration-header
// spacing without treating register modifier hyphens as expression operators.
// Rules: rules/tooling/formatter.md — §5(2–6),§6,§16(18–19),§22(1),§29.
func TestFormatCompleteHeaderSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/header_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/header_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{string(input), string(want)} {
		p := parser.New(lexer.New(source))
		p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		got := Format(Source{Text: source}, Options{}).Text
		if got != string(want) {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}
