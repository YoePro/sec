package formatter

import "testing"

// Minimal enclosing edits must reconstruct exact formatter bytes without
// splitting Unicode scalars or CRLF, including insertions and deletions.
// Rules: rules/tooling/formatter.md — § 28(2), § 29(2), (4), (7), (9).
func TestMinimalEdit(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		want                Edit
	}{
		{"insert", "abc", "ab c", Edit{2, 2, " "}},
		{"delete", "ab c", "abc", Edit{2, 3, ""}},
		{"empty source", "", "\n", Edit{0, 0, "\n"}},
		{"empty result", "\n", "", Edit{0, 1, ""}},
		{"shared rune prefix", "a😀z", "a😁z", Edit{1, 5, "😁"}},
		{"shared rune suffix", "aéz", "aĩz", Edit{1, 3, "ĩ"}},
		{"crlf suffix", "a\r\nb", "a\nb", Edit{1, 3, "\n"}},
		{"crlf prefix", "a\r\nb", "a\rb", Edit{1, 3, "\r"}},
		{"separated changes", "a x b y c", "a X b Y c", Edit{2, 7, "X b Y"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edit, changed := MinimalEdit(tc.before, tc.after)
			if !changed || edit != tc.want {
				t.Fatalf("edit = %#v, changed = %v; want %#v", edit, changed, tc.want)
			}
			if got := tc.before[:edit.Start] + edit.NewText + tc.before[edit.End:]; got != tc.after {
				t.Fatalf("applying edit = %q; want %q", got, tc.after)
			}
		})
	}
	for _, text := range []string{"", "canonical\r\n😀\n", "\xff"} {
		if _, changed := MinimalEdit(text, text); changed {
			t.Fatalf("unchanged %q produced an edit", text)
		}
	}
}
