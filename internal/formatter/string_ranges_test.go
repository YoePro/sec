package formatter

import (
	"os"
	"strings"
	"testing"
)

// Existing range syntax needs no grammar fork for MD-126 and must survive
// formatting and repeated formatting with its inclusive/exclusive semantics.
// Rules: rules/collections/collections.md — §§7.4,7.10;
// rules/tooling/formatter.md — §5 "Required syntax representation".
func TestStringRangeFormatting(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/string_ranges/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(Source{Text: string(source)}, Options{}).Text
	for _, text := range []string{"text[a..b]", "text[a..<b]", "text[..]", "view[..]", "try text[a..<b]"} {
		if !strings.Contains(formatted, text) {
			t.Fatalf("changed range %s:\n%s", text, formatted)
		}
	}
	if again := Format(Source{Text: formatted}, Options{}).Text; again != formatted {
		t.Fatal("string range formatting not idempotent")
	}
}
