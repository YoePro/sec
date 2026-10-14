package formatter

import "testing"

// MD-040/MD-046/MD-047 forms keep their canonical spelling: `impl <U>`,
// `<U>(value)`, `<U>.Member`, `bit[N](value)<U>` with no space before the
// suffix, a spaced comparison after an extraction, and a rounding argument.
//
// Rules:
//   - rules/tooling/formatter.md — operator and unit spacing
//   - rules/types/units.md — "Unit names and compiler-known names", "Explicit rounding"
func TestFormatKeepsQualifiedUnitForms(t *testing.T) {
	source := "module main\n\nunit rpm physical\n\nimpl <rpm> {\n    Dimension: [time^-1]\n    Scale: 1 / 60\n}\n\n" +
		"fn F(raw: uint16, v: decimal<rpm>) void {\n    let a := <rpm>(v)\n    let b := <rpm>.Symbol\n" +
		"    let c := bit[12](raw)<rpm>\n    let d := bit[8](raw) < 3\n    let e := try uint16<rpm>(v, RoundingMode.Ceil)\n}\n"
	assertFormat(t, source, source)
}
