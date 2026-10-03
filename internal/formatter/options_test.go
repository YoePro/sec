package formatter

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/tooling/formatter.md — § 4(5)–(8), § 4(14)
func TestFormatHonorsIndentationWidth(t *testing.T) {
	input := "fn F(value: int) int {\nif value > 0 {\nreturn 1\n}\nlet items := [1, 2] { }\nreturn 0\n}\nfn G() void { Use() }\n"
	for _, width := range []int{2, 4, 6} {
		got := Format(Source{Text: input}, Options{IndentationWidth: width}).Text
		one, two := strings.Repeat(" ", width), strings.Repeat(" ", 2*width)
		for _, want := range []string{"\n" + one + "if value > 0 {\n", "\n" + two + "return 1\n", "\n" + one + "Use()\n"} {
			if !strings.Contains(got, want) {
				t.Fatalf("width %d: output missing %q:\n%s", width, want, got)
			}
		}
		if again := Format(Source{Text: got}, Options{IndentationWidth: width}).Text; again != got {
			t.Fatalf("width %d is not idempotent:\n%s", width, again)
		}
	}
	if Format(Source{Text: input}, Options{}).Text != Format(Source{Text: input}, DefaultOptions()).Text {
		t.Fatal("zero options differ from the default configuration")
	}
}

func TestFormatAlignmentPaddingFollowsIndentationWidth(t *testing.T) {
	// Twelve spaces of padding fit four widths of 4 (16) but not of 2 (8).
	input := "type Pair struct {\n    a: int,\n    abcdefghijkl: string,\n}\n"
	if got := Format(Source{Text: input}, Options{IndentationWidth: 4}).Text; !strings.Contains(got, "a:            int") {
		t.Fatalf("width 4 did not align within its padding limit:\n%s", got)
	}
	if got := Format(Source{Text: input}, Options{IndentationWidth: 2}).Text; strings.Contains(got, "a:            int") {
		t.Fatalf("width 2 aligned beyond its eight-space padding limit:\n%s", got)
	}
}

// Rules:
//   - rules/tooling/formatter.md — § 13(8)–(9)
func TestFormatCompactImportRegionDropsClassSeparation(t *testing.T) {
	input := "module main\n\nimport sys \"platform/linux\"\nimport \"fmt\"\n"
	structured := Format(Source{Text: input}, Options{}).Text
	compact := Format(Source{Text: input}, Options{VerticalStyle: VerticalCompact}).Text
	if !strings.Contains(structured, "\"fmt\"\n\n    sys") || !strings.Contains(compact, "\"fmt\"\n    sys") {
		t.Fatalf("structured:\n%s\ncompact:\n%s", structured, compact)
	}
}

func TestValidateOptions(t *testing.T) {
	if err := ValidateOptions(DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOptions(Options{IndentationWidth: 3}); err == nil || !strings.Contains(err.Error(), "2, 4, or 6") {
		t.Fatalf("indentation width 3 = %v", err)
	}
	if err := ValidateOptions(Options{VerticalStyle: "dense"}); err == nil {
		t.Fatal("unknown vertical style accepted")
	}
}
