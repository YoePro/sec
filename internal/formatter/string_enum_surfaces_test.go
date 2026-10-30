package formatter

import (
	"os"
	"strings"
	"testing"
)

// TestStringEnumFrontendGolden preserves default/alias names, decoded literal
// spellings, switch case order and implicit default declarations on both passes.
// Rules: rules/declarations/enums.md — §§5,7–8,18;
// rules/tooling/formatter.md — §§15(4),18,29.
func TestStringEnumFrontendGolden(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_surfaces/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := string(data)
	input := strings.ReplaceAll(want, "Active default =", "Active    default=")
	input = strings.ReplaceAll(input, "Alias          =", "Alias=")
	for _, text := range []string{input, want} {
		got := Format(Source{Text: text}, Options{}).Text
		if got != want {
			t.Fatalf("string enum golden mismatch:\n%s\nwant:\n%s", got, want)
		}
	}
}
