package formatter

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// All parser-owned attribute targets, argument lists and modifier prefixes use
// one layout path; source order, comments and second-pass stability survive.
// Rules: rules/tooling/formatter.md §§16(14–18),22(5),25,29;
// rules/foundations/attributes.md — Attribute attachment.
func TestGeneralAttributeFormatting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/attributes/general.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{2, 4, 6} {
		options := Options{IndentationWidth: width}
		got := Format(Source{Text: string(data)}, options).Text
		p := parser.New(lexer.New(got))
		p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parse: %v\n%s", p.Errors(), got)
		}
		for _, want := range []string{
			"@target(os: \"linux\", arch: \"amd64\")\n@when(true)\n@noPanic\nunsafe fn Selected()",
			"@link_name(\"foreign_func\")\n@noAlloc\n@noBlock\nextern \"C\" fn Foreign",
			"@when(true)\nimport", "@when(true)\ninterface", "@when(true)\nunit", "@when(true)\nimpl",
			"@noPanic\n" + strings.Repeat(" ", width) + "init()", "@noPanic\n" + strings.Repeat(" ", width) + "free",
			"@noPanic\n" + strings.Repeat(" ", 2*width) + "get", "@noPanic\n" + strings.Repeat(" ", 2*width) + "set",
			"@noPanic\n" + strings.Repeat(" ", width) + "static fn Safe", "@noPanic\n" + strings.Repeat(" ", width) + "let callback",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q\n%s", want, got)
			}
		}
		if twice := Format(Source{Text: got}, options).Text; twice != got {
			t.Fatalf("not idempotent\n%s\n%s", got, twice)
		}
	}
}

// Long attribute argument lists follow the same structural comma-list rules
// as calls, retaining complete source syntax and attribute order.
// Rules: rules/tooling/formatter.md §§11(2–5),16(14–17),29.
func TestAttributeArgumentLayout(t *testing.T) {
	cases := []struct {
		source string
		width  uint64
		want   string
	}{
		{"module main\n@target(os: \"linux\", arch: \"amd64\") fn F() void {}\n", 24, "@target(\n    os: \"linux\",\n    arch: \"amd64\",\n)\nfn F()"},
		{"module main\n@target(\n os: \"linux\",\n arch: \"amd64\"\n) @noPanic fn F() void {}\n", 120, "@target(\n    os: \"linux\",\n    arch: \"amd64\",\n)\n@noPanic\nfn F()"},
		{"module main\n@target(os: \"linux\") // target\n@noPanic // effect\nfn F() void {}\n", 120, "@target(os: \"linux\") // target\n@noPanic // effect\nfn F()"},
	}
	for _, tc := range cases {
		got := Format(Source{Text: tc.source}, Options{LineWidth: tc.width}).Text
		if !strings.Contains(got, tc.want) {
			t.Fatalf("missing %q\n%s", tc.want, got)
		}
		if Format(Source{Text: got}, Options{LineWidth: tc.width}).Text != got {
			t.Fatal("not idempotent")
		}
	}
}

// Invalid attribute arguments retain their exact recovery region; formatting
// never repairs or reinterprets them as a complete valid attachment.
// Rules: rules/tooling/formatter.md §§5,25,29.
func TestMalformedAttributeFormatting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/attributes/list_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(data)}, Options{})
	for _, line := range []string{"@target(os: , arch: \"amd64\") fn F() void", "@noPanic(1) fn G() void"} {
		if !strings.Contains(got.Text, line) {
			t.Fatalf("invalid region changed: %s", got.Text)
		}
	}
}
