package formatter

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestFormatForBindingHeaders canonicalizes validated loop header tokens while
// preserving the binding mode, comments and range/step spelling.
// Rules: rules/control-flow/flowcontrol_for.md — §§4,6–7;
// rules/tooling/formatter.md — §§3(3–4),6,31(2,8).
func TestFormatForBindingHeaders(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/for_binding_headers.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/for_binding_headers.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{string(data), string(want)} {
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
