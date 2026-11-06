package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestFormatRawPointerConversions preserves locked raw/reference conversion spelling,
// authorization boundaries and tokens across every indentation configuration.
// Rules: rules/memory/raw_pointers.md §§9–10,17(7–9),18A,36(5);
// rules/tooling/formatter.md §§17,19(11–14),29(2).
func TestFormatRawPointerConversions(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/raw_pointer_conversions.sec")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("../../testdata/formatter/raw_pointer_conversions.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	before := rawPointerTokens(t, string(input))
	for _, width := range []int{2, 4, 6} {
		for _, crlf := range []bool{false, true} {
			for _, fix := range []bool{false, true} {
				t.Run(fmt.Sprintf("indent%d/crlf%t/fix%t", width, crlf, fix), func(t *testing.T) {
					source := string(input)
					if crlf {
						source = strings.ReplaceAll(source, "\n", "\r\n")
					}
					options := Options{IndentationWidth: width, Fix: fix}
					got := Format(Source{Text: source}, options)
					want := reindentOwnershipGolden(string(golden), width)
					if got.Malformed || got.Text != want {
						t.Fatalf("formatted (malformed=%t):\n%s\nwant:\n%s", got.Malformed, got.Text, want)
					}
					if after := rawPointerTokens(t, got.Text); !reflect.DeepEqual(before, after) {
						t.Fatalf("changed raw-pointer operations:\nbefore %v\nafter %v", before, after)
					}
					if again := Format(Source{Text: got.Text}, options); again.Malformed || again.Text != got.Text {
						t.Fatalf("unstable second pass:\n%s", again.Text)
					}
				})
			}
		}
	}
}

// A missing member is a parser recovery region, not a conversion the formatter
// may complete or turn into an implicit safe reference.
// Rules: rules/tooling/formatter.md §25; rules/memory/raw_pointers.md §36(5).
func TestFormatRawPointerConversionsRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/raw_pointer_conversion_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, fix := range []bool{false, true} {
		got := Format(Source{Text: string(data)}, Options{Fix: fix})
		if got.Text != string(data) {
			t.Fatalf("changed recovered conversion (fix=%t): %+v", fix, got)
		}
	}
}
