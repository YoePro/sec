package formatter

import (
	"strings"
	"testing"
)

func TestFormatPreservesDefaultClauseAndPartialStructLiteral(t *testing.T) {
	input := "module main\n\n" +
		"type User string in [\"Admin\", \"User\"] default \"User\"\n\n" +
		"fn main() int {\n" +
		"let mut user: User\n" +
		"let position := Position { line: 10 }\n" +
		"return 0\n" +
		"}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, `type User string in ["Admin", "User"] default "User"`) {
		t.Fatalf("formatter changed explicit default or membership order:\n%s", got)
	}
	if !strings.Contains(got, "Position { line: 10 }") {
		t.Fatalf("formatter expanded partial struct literal:\n%s", got)
	}
}

func TestFormatPlacesNoCopyAttributeOnOwnLine(t *testing.T) {
	input := "@noCopy type SessionID struct {\nvalue: uint64,\n}\n"
	want := "@noCopy\ntype SessionID struct {\n    value: uint64,\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong @noCopy formatting:\n%s\nwant:\n%s", got, want)
	}
}
