package formatter

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// Preferred width controls parser-owned single/mixed callable parameter lists;
// an explicitly multiline list stays multiline. Width is a column count, and
// long body expressions/literals are not split by this parameter-list policy.
// Rules: rules/tooling/formatter.md — §4(9–13),§10,§16(1–2),§29–30.
func TestPreferredWidthParameterLayout(t *testing.T) {
	source, err := os.ReadFile("../../testdata/formatter/preferred_width.sec")
	if err != nil {
		t.Fatal(err)
	}
	wide, err := os.ReadFile("../../testdata/formatter/preferred_width.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := os.ReadFile("../../testdata/formatter/preferred_width.narrow.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	header := strings.Split(string(wide), "\n")[2]
	limit := uint64(utf8.RuneCountInString(header))
	for _, input := range []string{string(source), string(wide), strings.ReplaceAll(string(source), "\n", "\r\n")} {
		for _, width := range []uint64{1, 64, limit - 1, limit, limit + 1, 120, 0} {
			want := string(wide)
			if width != 0 && width < limit {
				want = string(narrow)
			}
			options := Options{LineWidth: width}
			got := Format(Source{Text: input}, options)
			if got.Malformed || got.Text != want {
				t.Fatalf("width %d:\n%s\nwant:\n%s", width, got.Text, want)
			}
			if again := Format(Source{Text: got.Text}, options); again.Text != got.Text || again.Malformed {
				t.Fatal("width-dependent formatting is not a fixed point", width, again.Text)
			}
		}
	}
	if got := Format(Source{Text: string(narrow)}, Options{LineWidth: 1000}).Text; got != string(narrow) {
		t.Fatal("wide viewport destroyed deliberate multiline layout", got)
	}
	// Derive a Unicode identifier from the same real source fixture; byte
	// length differs from the canonical column count by one.
	unicodeWide := strings.ReplaceAll(string(wide), "CombineThreeValues", "ÅombineThreeValues")
	unicodeLimit := uint64(utf8.RuneCountInString(strings.Split(unicodeWide, "\n")[2]))
	if got := Format(Source{Text: unicodeWide}, Options{LineWidth: unicodeLimit}).Text; got != unicodeWide {
		t.Fatal("UTF-8 byte count replaced column count", got)
	}
	for _, indentation := range []int{2, 4, 6} {
		// Source indentation deliberately differs from the requested width.
		// The width decision must use the eventual structural columns.
		nested := "module width\n\ntype Holder struct {}\nimpl Holder {\n" + strings.Join(strings.Split(string(wide), "\n")[2:], "\n") + "}\n"
		for _, width := range []uint64{limit + uint64(indentation) - 1, limit + uint64(indentation)} {
			options := Options{LineWidth: width, IndentationWidth: indentation}
			got := Format(Source{Text: nested}, options).Text
			broken := strings.Contains(got, "fn CombineThreeValues(\n")
			if broken != (width < limit+uint64(indentation)) {
				t.Fatal("noncanonical indentation determined wrapping", indentation, width, got)
			}
			if again := Format(Source{Text: got}, options).Text; again != got {
				t.Fatal("nested width decision changed on second pass", indentation, width, again)
			}
		}
	}
}
