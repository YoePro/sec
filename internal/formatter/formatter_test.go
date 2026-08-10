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

func TestFormatRemovesInitialByteOrderMark(t *testing.T) {
	got := Format(Source{Text: "\uFEFFmodule main\n"}, Options{}).Text
	if got != "module main\n" {
		t.Fatalf("formatter retained initial BOM: %q", got)
	}
}

func TestFormatImplExtension(t *testing.T) {
	input := "impl extends Vehicle {\nfn Stop() void {\n}\n}\n"
	want := "impl extends Vehicle {\n    fn Stop() void {\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong impl extension formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesCanonicalNumericFamilySuffixes(t *testing.T) {
	input := "fn Values() void {\nlet values := [8i, 8u, 8g, 8m, 65t, 65r, 0x41t, 0x10g, 0x10m]\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, literal := range []string{"8i", "8u", "8g", "8m", "65t", "65r", "0x41t", "0x10g", "0x10m"} {
		if !strings.Contains(got, literal) {
			t.Fatalf("formatter lost canonical literal %s:\n%s", literal, got)
		}
	}
}
