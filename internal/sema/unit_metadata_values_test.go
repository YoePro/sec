package sema

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema/constant"
)

// A Scale read projects the exact rational to the nearest decimal with ties
// to even, in one rounding; exactly representable values keep their shortest
// spelling, and the finer lattice's largest coefficient wins when closer.
// The scale is a signed int32 (MD-066): a large value takes a negative scale
// instead of failing, and the coefficient domain is the asymmetric signed one.
// Rules: rules/types/units.md — "Unit metadata value types" (7);
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 9(7);
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 2(2), 3(5).
func TestNearestDecimalProjection(t *testing.T) {
	rat := func(text string) *big.Rat {
		value, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatalf("bad rational %s", text)
		}
		return value
	}
	for _, tc := range []struct {
		value, want string
		projected   bool
	}{
		{"1/2", "0.5", false},
		{"1000", "1000", false},
		{"1/3", "0.3333333333333333333", true},
		{"-1/3", "-0.3333333333333333333", true},
		{"2/3", "0.6666666666666666667", true},
		{"12345678901234567895/100000000000000000000", "0.123456789012345679", true},
		{"12345678901234567885/100000000000000000000", "0.1234567890123456788", true},
		{"55340232221128654847/60", "922337203685477580.7", true},
		{"1000000000000000000000000000000", "1000000000000000000e12", false},
		{"-9223372036854775808", "-9223372036854775808", false},
		{"-92233720368547758080", "-9223372036854775808e1", false},
		{"100000000000000000000/3", "3333333333333333333e1", true},
	} {
		text, projected, ok := constant.NearestDecimalText(rat(tc.value), 64)
		if !ok || text != tc.want || projected != tc.projected {
			t.Errorf("%s: got %q projected=%v ok=%v, want %q projected=%v", tc.value, text, projected, ok, tc.want, tc.projected)
		}
	}
	coefficient, scale, projected, ok := constant.NearestDecimal(rat("1000000000000000000000000000000"), 64)
	if !ok || projected || scale != -12 || coefficient.String() != "1000000000000000000" {
		t.Errorf("10^30 = %v × 10^-(%d) projected=%v ok=%v", coefficient, scale, projected, ok)
	}
}

// Reads record their resolved values: omission defaults, the declared
// category, and only Scale may be a projection; other decimal members that
// are not exactly representable fail the read, not the declaration.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (1), (7), (10)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(7)–9(8), 10(1), 10(3)
func TestUnitMetadataReadValues(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
unit plain
unit third physical
impl <third> {
    Dimension: [length^1]
    Scale: 1 / 3
}
fn Read() void {
    let category := <plain>.Category
    let kind := <plain>.Kind
    let scale := <third>.Scale
    let offset := <third>.Offset
}
`)
	assertSemaErrors(t, errors, nil)
	reads := map[string]UnitMetadataRead{}
	for _, read := range analyzer.unitMetadataReads {
		reads["<"+read.Unit+">."+read.Member] = read
	}
	if read := reads["<plain>.Category"]; read.Enum.Name != "Other" || read.Defaulted {
		t.Errorf("<plain>.Category = %+v", read)
	}
	if read := reads["<plain>.Kind"]; read.Enum.Name != "Unknown" || read.Enum.EnumType.Name != "UnitKind" || !read.Defaulted {
		t.Errorf("<plain>.Kind = %+v", read)
	}
	if read := reads["<third>.Scale"]; !read.Projected || read.Exact.RatString() != "1/3" || read.Defaulted {
		t.Errorf("<third>.Scale = %+v", read)
	}
	if read := reads["<third>.Offset"]; read.Decimal != "0" || read.Projected || !read.Defaulted {
		t.Errorf("<third>.Offset = %+v", read)
	}

	errors = analyzeSource(t, `module main
unit skew physical
impl <skew> {
    Dimension: [thermodynamic_temperature^1]
    Transform: UnitTransform.Affine
    Scale: 1
    Offset: 1 / 3
    Origin: UnitOrigin.AbsoluteZero
}
fn Read() decimal {
    return <skew>.Offset
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "Offset metadata 1/3 of unit <skew> cannot be represented as decimal") {
		t.Fatalf("want one representability error, got %+v", errors)
	}
}

// `<U>.Kind` stays a metadata read when the unit's custom Kind enum has a
// member spelled Kind: it is never resolved as that enum constant.
// Rules: rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 11(2).
func TestUnitMetadataReadIsNotEnumCase(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main
enum Domain {
    Kind,
    Flow,
}
unit flow other
impl <flow> {
    Kind: Domain.Flow
}
fn Read() Domain {
    return <flow>.Kind
}
`)
	assertSemaErrors(t, errors, nil)
	for expr, read := range analyzer.unitMetadataReads {
		if _, isCase := analyzer.ResolvedEnumCaseOf(expr); isCase {
			t.Fatalf("<%s>.%s resolved as an enum case", read.Unit, read.Member)
		}
		if read.Enum.Name != "Flow" || read.Enum.Ordinal != 1 {
			t.Fatalf("<flow>.Kind = %+v", read.Enum)
		}
	}
}
