package formatter

import (
	"os"
	"testing"
)

// TestFormatSpawnForms preserves contextual kinds and expression/block forms
// at canonical indentation, without rewriting valid grammar into call syntax.
// Rules: rules/concurrency/spawn.md — §2; rules/foundations/grammar.md — §89;
// rules/tooling/formatter.md — §§5,17,25.
func TestFormatSpawnForms(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/spawn_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/spawn_forms.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(Source{Text: string(input)}, Options{})
	if got.Malformed || got.Text != string(want) {
		t.Fatalf("malformed=%t\n%s", got.Malformed, got.Text)
	}
	if again := Format(Source{Text: got.Text}, Options{}); again.Malformed || again.Text != got.Text {
		t.Fatal(again)
	}
}
