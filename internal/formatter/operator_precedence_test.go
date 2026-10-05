package formatter

import "testing"

// Contextual infix spellings use the same canonical precedence catalog as the
// parser, including the complete two-token `not in` operator extent.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence", "Membership operators `in` and `not in`"
//   - rules/tooling/formatter.md — § 9(13)–(15) multiline infix expressions
func TestFormatMultilineContextualInfixUsesCanonicalPrecedence(t *testing.T) {
	input := "fn F(a: int, b: int, c: int, values: int[2]) bool {\n    let product :=\n        a\n        x b\n        x c\n    let included :=\n        a\n        in values\n    return included\n}\n"
	want := "fn F(a: int, b: int, c: int, values: int[2]) bool {\n    let product :=\n          a\n        x b\n        x c\n    let included :=\n          a\n       in values\n    return included\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}
