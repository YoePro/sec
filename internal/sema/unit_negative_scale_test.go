package sema

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema/constant"
)

const negativeScaleUnits = `module main
unit up physical
impl <up> {
    Dimension: [length^1]
    Kind: UnitKind.Length
}
unit down physical
impl <down> {
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: -1
}
unit half physical
impl <half> {
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: -1 / 2
}
`

// A linear Scale may be negative but never zero; a negative affine Scale
// waits for MD-048 and is rejected.
//
// Rules:
//   - rules/types/units.md — "Unit metadata" `Scale`, "Validation of metadata"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(2), 13(4), 14(4)
func TestUnitScaleIsNonZero(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, negativeScaleUnits+"fn Use() void {\n    let scale: decimal := <down>.Scale\n}\n")
	assertSemaErrors(t, errors, nil)
	if scale := analyzer.units["half"].ScaleValue; scale == nil || scale.RatString() != "-1/2" {
		t.Fatalf("exact negative scale = %v", scale)
	}
	for _, tc := range []struct{ metadata, want string }{
		{"Dimension: [length^1]\n    Scale: 0", "invalid scale metadata for unit bad: Scale must be non-zero"},
		{"Dimension: [length^1]\n    Scale: 1 - 1", "invalid scale metadata for unit bad: Scale must be non-zero"},
		{"Dimension: [thermodynamic_temperature^1]\n    Transform: UnitTransform.Affine\n    Scale: -1\n    Offset: 273.15\n    Origin: UnitOrigin.AbsoluteZero", "affine unit bad cannot have a negative Scale"},
	} {
		errors := analyzeSource(t, "module main\nunit bad physical\nimpl <bad> {\n    "+tc.metadata+"\n}\n")
		if len(errors) != 1 || !strings.Contains(errors[0].Message, tc.want) {
			t.Errorf("want exactly %q, got %+v", tc.want, errors)
		}
	}
}

// Implicit conversions through a negative factor are planned with the
// converted interval reversed: a value range is proven against the carrier
// after its endpoints swap, so a reversed interval cannot slip past the
// range check, and unsigned carriers reject negative results.
//
// Rules:
//   - rules/types/units.md — "Exact fixed conversions", "No hidden precision loss", "Unit metadata" `Scale`
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(2), 12(5), 13(4), 14(4)
func TestNegativeScaleConversionIntervals(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"constant fits", "let source: int8<up> := 100\n    let target: int8<down> := source", true},
		{"constant overflows after negation", "let source: int8<up> := -128\n    let target: int8<down> := source", false},
		{"reversed interval overflows", "if value <= 0 {\n        let target: int8<down> := value\n    }", false},
		{"reversed interval fits", "if value >= -127 && value <= 0 {\n        let target: int8<down> := value\n    }", true},
		{"unsigned negative result", "let source: uint8<up> := 5\n    let target: uint8<down> := source", false},
		{"unsigned zero", "let source: uint8<up> := 0\n    let target: uint8<down> := source", true},
		{"fractional single value", "let source: int8<up> := 6\n    let target: int8<half> := source", true},
		{"decimal", "let source: decimal<up> := 1.5\n    let target: decimal<down> := source", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errors := analyzeSource(t, negativeScaleUnits+"fn Use(value: int8<up>) void {\n    "+tc.body+"\n}\n")
			if tc.valid && len(errors) != 0 || !tc.valid && len(errors) == 0 {
				t.Fatalf("valid=%v, got %+v", tc.valid, errors)
			}
		})
	}
}

// The binary floating exactness proof handles a negative factor by mirroring
// the source interval; a zero factor is never proven.
//
// Rules:
//   - rules/types/units.md — "Exact fixed conversions", "No hidden precision loss"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 12(5)
func TestFloatAffineIntervalExactWithNegativeFactor(t *testing.T) {
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	for _, tc := range []struct {
		lower, upper, factor *big.Rat
		want                 bool
	}{
		{r(1, 1), r(2, 1), r(-1, 1), true},
		{r(-4, 1), r(4, 1), r(-2, 1), true},
		{r(1, 1), r(2, 1), r(-1, 3), false},
		{r(1, 1), r(2, 1), r(0, 1), false},
	} {
		got := constant.FloatAffineIntervalExact(tc.lower, tc.upper, tc.factor, r(0, 1), 64)
		mirrored := tc.factor.Sign() != 0 && constant.FloatAffineIntervalExact(new(big.Rat).Neg(tc.upper), new(big.Rat).Neg(tc.lower), new(big.Rat).Neg(tc.factor), r(0, 1), 64)
		if got != tc.want || tc.factor.Sign() != 0 && got != mirrored {
			t.Errorf("[%s, %s] * %s = %v (mirrored %v), want %v", tc.lower, tc.upper, tc.factor, got, mirrored, tc.want)
		}
	}
	errors := analyzeSource(t, negativeScaleUnits+"fn Use() void {\n    let source: float64<up> := 1.5\n    let target: float64<down> := source\n}\n")
	assertSemaErrors(t, errors, nil)
}

// A time unit with a negative Scale converts to duration with its bounded
// nanosecond interval in ascending order.
//
// Rules:
//   - rules/types/temporal.md — § 5.6; rules/types/units.md — "Unit metadata" `Scale`
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(2), 12(5)
func TestNegativeScaleDurationConversion(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
unit back physical
impl <back> {
    Dimension: [time^1]
    Scale: -1
}
fn Use(value: int8<back>) duration {
    return duration(value)
}
`)
	assertSemaErrors(t, errors, nil)
	found := false
	for _, plan := range analyzer.durationConversions {
		if plan.Proof == DurationConversionBoundedDomain {
			found = true
			if plan.Minimum.Cmp(plan.Maximum) > 0 || plan.Minimum.String() != "-127000000000" || plan.Maximum.String() != "128000000000" {
				t.Fatalf("bounded duration interval = %s..%s", plan.Minimum, plan.Maximum)
			}
		}
	}
	if !found {
		t.Fatal("no bounded duration plan")
	}
}
