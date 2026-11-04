package formatter

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// A single authored declaration gap survives either style, including the
// Abs/Min/Max/static Parse method sequence and each aggregate declaration kind.
// Rules: rules/tooling/formatter.md — §7(1),§7(4),§7(13).
func TestDeclarationBlankLines(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/declaration_blank_lines.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []VerticalStyle{VerticalCompact, VerticalStructured} {
		for _, width := range []int{2, 4, 6} {
			for _, extra := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/extra%t", style, width, extra), func(t *testing.T) {
					text := string(data)
					if extra {
						text = strings.ReplaceAll(text, "\n\n", "\n\n\n\n")
					}
					options := Options{VerticalStyle: style, IndentationWidth: width}
					got := Format(Source{Text: text}, options)
					if got.Malformed {
						t.Fatal("invalid source fixture")
					}
					for _, name := range []string{"fn Min(", "fn Max(", "static fn Parse("} {
						if !strings.Contains(got.Text, "\n\n"+strings.Repeat(" ", width)+name) {
							t.Fatalf("removed method separation before %s:\n%s", name, got.Text)
						}
					}
					for _, name := range []string{"impl Number", "fn BeforeStruct", "type Record", "fn BeforeEnum", "enum Mode", "fn BeforeUnion", "type Choice", "fn BeforeRegister", "type Flags", "fn Last"} {
						if !strings.Contains(got.Text, "\n\n"+name) {
							t.Fatalf("removed declaration separation before %s:\n%s", name, got.Text)
						}
					}
					if strings.Contains(got.Text, "\n\n\n") {
						t.Fatal("multiple blank lines were not collapsed")
					}
					if again := Format(Source{Text: got.Text}, options); again.Text != got.Text {
						t.Fatal("second pass removed separation")
					}
				})
			}
		}
	}
}
