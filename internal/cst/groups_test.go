package cst

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// rules/tooling/formatter.md "Source model" requires structural source ranges
// to preserve literal and comment text while retaining nested delimiter facts.
func TestBuildGroupsRealNestedDelimiters(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/cst/groups_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(bytes), "\n", "\r\n")
	document := Build(source, "groups.sec")
	if got := document.Text(); got != source {
		t.Fatalf("round trip = %q, want %q", got, source)
	}
	if len(document.Diagnostics) != 0 || len(document.UnmatchedClosers) != 0 || len(document.Groups) != 4 {
		t.Fatalf("groups = %+v, unmatched = %+v, diagnostics = %+v", document.Groups, document.UnmatchedClosers, document.Diagnostics)
	}
	wants := []struct {
		open   lexer.TokenType
		close  lexer.TokenType
		parent int
	}{
		{lexer.LPAREN, lexer.RPAREN, -1},
		{lexer.LBRACE, lexer.RBRACE, -1},
		{lexer.LPAREN, lexer.RPAREN, 1},
		{lexer.LBRACKET, lexer.RBRACKET, 2},
	}
	for index, want := range wants {
		group := document.Groups[index]
		open := document.Elements[group.Open]
		close := document.Elements[group.Close]
		if open.Token.Type != want.open || close.Token.Type != want.close || group.Parent != want.parent ||
			group.Span.Start != open.Span.Start || group.Span.End != close.Span.End {
			t.Errorf("group %d = %+v (%s, %s), want (%s, %s) parent %d", index, group, open.Token.Type, close.Token.Type, want.open, want.close, want.parent)
		}
	}
}

// A mismatched closer is kept as a real token. This lexical view does not
// fabricate a missing delimiter or borrow a closer from an outer group.
func TestBuildGroupsRetainsIncompleteAndMismatchedDelimiters(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/cst/groups_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	document := Build(source, "incomplete.sec")
	if document.Text() != source || len(document.Groups) != 2 || len(document.UnmatchedClosers) != 1 {
		t.Fatalf("source or grouping lost: %+v", document)
	}
	outer, inner := document.Groups[0], document.Groups[1]
	if outer.Close != -1 || outer.Parent != -1 || outer.Span.End != len(source) ||
		inner.Parent != 0 || inner.Close < 0 || document.Elements[inner.Close].Token.Type != lexer.RBRACKET {
		t.Fatalf("outer = %+v, inner = %+v", outer, inner)
	}
	unmatched := document.Elements[document.UnmatchedClosers[0]]
	if unmatched.Token.Type != lexer.RPAREN || unmatched.Text != ")" {
		t.Fatalf("unmatched closer = %+v", unmatched)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("lexer diagnostics = %+v, want no syntax-level diagnostic", document.Diagnostics)
	}
}
