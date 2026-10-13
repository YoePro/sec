package formatter

import "testing"

// A quantity literal keeps its canonical adjacent spelling, so `500<ms>` is
// never split into a comparison chain, while an ordinary comparison keeps its
// spaces.
//
// Rules:
//   - rules/types/units.md — "Quantity literals"
//   - rules/tooling/formatter.md — operator spacing
func TestFormatKeepsQuantityLiteralsAdjacent(t *testing.T) {
	source := "module main\n\nfn F(a: int, b: int) void {\n" +
		"    let x := 5<s> + 2<ms>\n    let y := a < b\n    let z := -0.5<ns>\n" +
		"    let w := 100<m/s^2>\n    let v := duration(1<us>)\n}\n"
	assertFormat(t, source, source)
}
