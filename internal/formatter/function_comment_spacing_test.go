package formatter

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Required function/comment separation is independent of style and whether a
// following declaration exists. Comment attachment and second-pass output stay
// stable, with or without a blank line in the original input.
// Rules: rules/tooling/formatter.md — §7(7a),§12,§29.
func TestFunctionCommentSpacing(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/function_comment_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []VerticalStyle{VerticalCompact, VerticalStructured} {
		for _, gap := range []string{"\n", "\n\n", "\n\n\n\n"} {
			for _, crlf := range []bool{false, true} {
				text := strings.ReplaceAll(string(input), "}\n\n", "}\n")
				text = strings.ReplaceAll(text, "}\n", "}"+gap)
				if crlf {
					text = strings.ReplaceAll(text, "\n", "\r\n")
				}
				before := verticalDocumentation(t, text)
				options := Options{VerticalStyle: style}
				got := Format(Source{Text: text}, options)
				if got.Malformed || got.Text != string(golden) {
					t.Fatalf("%s gap=%q crlf=%t:\n%s\nwant:\n%s", style, gap, crlf, got.Text, golden)
				}
				if !reflect.DeepEqual(before, verticalDocumentation(t, got.Text)) {
					t.Fatal("comment attachment changed")
				}
				if again := Format(Source{Text: got.Text}, options); again.Text != got.Text {
					t.Fatal("second pass changed separation")
				}
			}
		}
	}
}
