package formatter

import "testing"

// grammar.md § 126(1): the formatter recognizes the standalone @cAlias
// declaration, normalizes argument spacing, keeps the author's single-line
// or one-argument-per-line layout and is stable on a second pass.
func TestFormatCAliasDeclarations(t *testing.T) {
	input := "module amd64\n\n@cAlias( name:\"c::stddef::size_t\",target:c::ulong )\n\n@cAlias(\n  name: \"c::example::pointer\", // pointer to char\n      target: RawPtr[ c::char ],\n)\n"
	want := "module amd64\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n\n@cAlias(\n    name: \"c::example::pointer\", // pointer to char\n    target: RawPtr[c::char],\n)\n"
	result := Format(Source{Text: input}, Options{})
	if result.Malformed || result.Text != want {
		t.Fatalf("formatted (malformed %v):\n%s\nwant:\n%s", result.Malformed, result.Text, want)
	}
	if again := Format(Source{Text: result.Text}, Options{}).Text; again != result.Text {
		t.Fatalf("second pass changed:\n%s", again)
	}
}
