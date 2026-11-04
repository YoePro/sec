package main

import (
	"os"
	"testing"
)

// Existing method/type separation is preserved independently of vertical style.
// Rules: rules/tooling/formatter.md — §7(4),§28.
func TestFmtDeclarationBlankLines(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/declaration_blank_lines.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		_, file := writeFmtProject(t, "[format]\nvertical_style = \""+style+"\"\n", string(input))
		if err := runFmtCommand([]string{file}); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, file); got != string(input) {
			t.Fatalf("%s removed authored declaration spacing:\n%s", style, got)
		}
		if err := runFmtCommand([]string{"--check", file}); err != nil {
			t.Fatal(err)
		}
	}
}

// sec fmt consumes the project's vertical style and accepts its own canonical
// output in check mode; both styles use the shared formatter's exact golden.
// Rules: rules/tooling/formatter.md — §4(6–7,22),§7(3–13),§28.
func TestFmtVerticalSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/vertical_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		name := "vertical_spacing.expected.sec"
		if style == "compact" {
			name = "vertical_spacing.compact.expected.sec"
		}
		want, err := os.ReadFile("../../testdata/formatter/" + name)
		if err != nil {
			t.Fatal(err)
		}
		_, file := writeFmtProject(t, "[format]\nvertical_style = \""+style+"\"\n", string(input))
		if err := runFmtCommand([]string{file}); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, file); got != string(want) {
			t.Fatalf("%s CLI differs from shared golden:\n%s", style, got)
		}
		if err := runFmtCommand([]string{"--check", file}); err != nil {
			t.Fatal(err)
		}
	}
}

// Required separation survives the project compact style and CLI check mode.
// Rules: rules/tooling/formatter.md — §7(7a),§28.
func TestFmtFunctionCommentSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"structured", "compact"} {
		_, file := writeFmtProject(t, "[format]\nvertical_style = \""+style+"\"\n", string(input))
		if err := runFmtCommand([]string{file}); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, file); got != string(want) {
			t.Fatalf("%s CLI removed required comment separation:\n%s", style, got)
		}
		if err := runFmtCommand([]string{"--check", file}); err != nil {
			t.Fatal(err)
		}
	}
}
