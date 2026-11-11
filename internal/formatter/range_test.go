package formatter

import (
	"os"
	"strings"
	"testing"
)

// Selected edits retain full-context layout and exact unselected bytes.
// Rules: rules/tooling/formatter.md §§5,9,12,25,28,29; rules/tooling/lsp.md Appendix A.8.
func TestRangeFormatting(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_selection.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/range_selection.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, newline := range []string{"\n", "\r\n"} {
		text := strings.ReplaceAll(string(data), "\n", newline)
		start := strings.Index(text, "return 1+2")
		end := start + len("return 1+2")
		edit, changed := RangeEdit(Source{Text: text}, DefaultOptions(), start, end)
		if !changed {
			t.Fatal("selected declaration was not formatted")
		}
		got := text[:edit.Start] + edit.NewText + text[edit.End:]
		want := string(golden)
		if newline == "\r\n" {
			// Only the selected unit gets the canonical LF line endings.
			unitStart := strings.Index(text, "/** Focus")
			unitEnd := strings.Index(text, "\r\n\r\nfn After") + 2
			formatted := string(golden)[strings.Index(string(golden), "/** Focus") : strings.Index(string(golden), "\n\nfn After")+1]
			want = text[:unitStart] + formatted + text[unitEnd:]
		}
		if got != want {
			t.Fatalf("range output:\n%q\nwant:\n%q", got, want)
		}
		at := strings.Index(got, "return 1 + 2")
		if edit, changed := RangeEdit(Source{Text: got}, DefaultOptions(), at, at); changed {
			t.Fatal("not idempotent", edit)
		}
		full, changed := RangeEdit(Source{Text: text}, DefaultOptions(), 0, len(text))
		if !changed || text[:full.Start]+full.NewText+text[full.End:] != Format(Source{Text: text}, DefaultOptions()).Text {
			t.Fatal("whole-range differs from document formatting")
		}
	}
}

// Nested declarations preserve their enclosing alignment/indentation context;
// imports are selected as one sortable group. Uncertainty never makes an edit.
// Rules: rules/tooling/formatter.md §§5,9,12,13,25,29.
func TestRangeFormattingStructuralBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_selection.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, selected := range []string{"return self.x", "longName:int"} {
		at := strings.Index(text, selected)
		edit, changed := RangeEdit(Source{Text: text}, Options{IndentationWidth: 2}, at, at+len(selected))
		if !changed {
			t.Fatal(selected)
		}
		got := text[:edit.Start] + edit.NewText + text[edit.End:]
		if strings.Contains(selected, "self") {
			if !strings.Contains(got, "\n  fn Read() int {\n    return self.x\n  }\n\n  fn Twice() int {") || !strings.Contains(got, "fn Before( )") {
				t.Fatal(got)
			}
		} else if !strings.Contains(got, "\n  x:        int,\n  longName: int,\n") {
			t.Fatal(got)
		}
	}
	for _, suffix := range []string{"\nfn Incomplete() void {", "\nfn Invalid( {}", "\n`", "\nlet broken :="} {
		invalid := text + suffix
		if edit, changed := RangeEdit(Source{Text: invalid}, DefaultOptions(), strings.Index(text, "Focus"), strings.Index(text, "Focus")+5); changed {
			t.Fatal("uncertain source edited", edit)
		}
	}
	for _, bounds := range [][2]int{{-1, 0}, {4, 3}, {0, len(text) + 1}, {strings.Index(text, "😀") + 1, strings.Index(text, "😀") + 2}} {
		if _, changed := RangeEdit(Source{Text: text}, DefaultOptions(), bounds[0], bounds[1]); changed {
			t.Fatal("invalid boundary edited", bounds)
		}
	}
}

// Sorting selected imports must include the whole group without duplicating,
// losing or formatting an unrelated declaration.
// Rules: rules/tooling/formatter.md §§5,13,28,29.
func TestRangeFormattingImports(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/range_imports.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	at := strings.Index(text, "zeta")
	edit, changed := RangeEdit(Source{Text: text}, DefaultOptions(), at, at+1)
	if !changed {
		t.Fatal("no import edit")
	}
	got := text[:edit.Start] + edit.NewText + text[edit.End:]
	if !strings.Contains(got, "import (\n    \"alpha\"\n    \"zeta\"\n)") || strings.Count(got, "alpha") != 1 || strings.Count(got, "zeta") != 1 || !strings.HasSuffix(got, "fn Unselected( ) int { return  1 }\n") {
		t.Fatal(got)
	}
	at = strings.Index(got, "alpha")
	if edit, changed := RangeEdit(Source{Text: got}, DefaultOptions(), at, at+1); changed {
		t.Fatal("second import pass", edit)
	}
}
