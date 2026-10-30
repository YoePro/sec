package formatter

import (
	"strings"
	"testing"
)

// The control-condition correction removes only one pair that encloses the
// whole condition. A condition that merely begins with `(` and ends with `)`
// keeps its parentheses, as does a group whose removal would let a brace inside
// it become the body of the statement.
// Rules: rules/tooling/formatter.md — §§ 17(13), 27(17)–(18).
func TestControlConditionCorrectionNeedsOneEnclosingPair(t *testing.T) {
	for _, test := range []struct{ name, header, want string }{
		{"masked call", "if (first & byte(0xC0)) == byte(0x80) {", ""},
		{"two groups", "if (a) && (b) {", ""},
		{"group then group while", "while (x) == (y) {", ""},
		{"call on the right while", "while (x) == int(y) {", ""},
		{"member call", "if (a).Ok() {", ""},
		{"called group", "if (a)(b) {", ""},
		{"negated group", "if !(a) {", ""},
		{"try handler group", "if (try F() { Err(_) => false }) {", ""},
		{"struct literal group", "if (p == P { X: 1 }) {", ""},
		{"lambda group", "if (fn() bool { return true })() {", "if (fn() bool {"},
		{"whole name", "if (a) {", "if a {"},
		{"whole expression", "if (a && b) {", "if a && b {"},
		{"whole with call", "if (f(a) == f(b)) {", "if f(a) == f(b) {"},
		{"whole nested groups", "if ((a) && (b)) {", "if (a) && (b) {"},
		{"whole while", "while (a) {", "while a {"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := "fn Check() void {\n    " + test.header + "\n        return\n    }\n}\n"
			want := test.want
			if want == "" {
				want = test.header
			}
			got := Format(Source{Text: input}, Options{Fix: true}).Text
			if !strings.Contains(got, "    "+want+"\n") {
				t.Fatalf("got:\n%s\nwant header %q", got, want)
			}
			if again := Format(Source{Text: got}, Options{Fix: true}).Text; again != got {
				t.Fatalf("not idempotent:\n%s", again)
			}
		})
	}
	for _, test := range []struct{ name, header, want string }{
		{"two groups", "switch (a) + (b) {", ""},
		{"call on the right", "switch (a) + f(b) {", ""},
		{"whole", "switch (a + b) {", "switch a + b {"},
	} {
		t.Run("switch "+test.name, func(t *testing.T) {
			input := "fn Check() void {\n    " + test.header + "\n    default:\n        return\n    }\n}\n"
			want := test.want
			if want == "" {
				want = test.header
			}
			if got := Format(Source{Text: input}, Options{Fix: true}).Text; !strings.Contains(got, "    "+want+"\n") {
				t.Fatalf("got:\n%s\nwant header %q", got, want)
			}
		})
	}
}
