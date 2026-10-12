package cst

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestSingleLineDelimiterRolesRequireParserConfirmation checks lossless
// projection and local comment/line boundaries with one canonical grammar.
// Rules: rules/tooling/formatter.md — §5(1–6),§8,§11,§12,§25.
func TestSingleLineDelimiterRolesRequireParserConfirmation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/cst_delimiter_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	p := parser.New(lexer.New(text))
	p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	for _, confirmed := range []bool{false, true} {
		document := Build(text, "")
		document.ApplySingleLineDelimiterRoles(confirmed)
		if document.Text() != text {
			t.Fatal("role projection changed source bytes")
		}
		count := 0
		for _, element := range document.Elements {
			if element.HasRole(CompactDelimiterOpen) || element.HasRole(PaddedDelimiterOpen) {
				count++
			}
		}
		if confirmed && count == 0 || !confirmed && count != 0 {
			t.Fatalf("confirmed=%t role count=%d", confirmed, count)
		}
		for _, group := range document.Groups {
			if document.Elements[group.Open].Token.Line != document.Elements[group.Close].Token.Line && (document.Elements[group.Open].HasRole(CompactDelimiterOpen) || document.Elements[group.Open].HasRole(PaddedDelimiterOpen)) {
				t.Fatal("multiline group marked single-line")
			}
			for index := group.Open + 1; index < group.Close; index++ {
				if document.Elements[index].Kind == Comment && (document.Elements[group.Open].HasRole(CompactDelimiterOpen) || document.Elements[group.Open].HasRole(PaddedDelimiterOpen)) {
					t.Fatal("comment-bearing group marked single-line")
				}
			}
		}
	}
}
