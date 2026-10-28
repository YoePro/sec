package semantic

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

const unitMetadataIRUnits = `module main
enum Domain {
    Kind,
    Flow,
}
unit m physical
impl <m> {
    LongName: "Metre"
    Symbol: "m"
    BaseUnit: true
    Dimension: [length^1]
    Kind: UnitKind.Length
    System: UnitSystem.SI
}
unit km physical
impl <km> {
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: 1000
}
unit minute physical
impl <minute> {
    Dimension: [time^1]
    Scale: 60
}
unit third physical
impl <third> {
    Dimension: [length^1]
    Scale: 1 / 3
}
unit flow other
impl <flow> {
    Kind: Domain.Flow
}
unit giga physical
impl <giga> {
    Dimension: [length^1]
    Scale: 1000000000000000000000000000000
}
unit bel ratio
impl <bel> {
    Transform: UnitTransform.Logarithmic
    LogBase: 10
    LogFactor: 1
    Reference: 1
}
unit celsius physical
impl <celsius> {
    Dimension: [thermodynamic_temperature^1]
    Transform: UnitTransform.Affine
    Scale: 1
    Offset: 273.15
    Origin: UnitOrigin.AbsoluteZero
}
`

// Unit metadata reads fold to typed constants that keep the unit, member and
// exact-versus-projected decimal provenance; the IR verifies and prints them.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types", "Semantic IR and lowering"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(12), 9(7), 11(5), 12(7)
func TestUnitMetadataReadsFoldToConstants(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"fn Read() string {\n    return <m>.LongName\n}\n", `const.string "Metre" unit-metadata=<m>.LongName`},
		{"fn Read() string {\n    return <km>.Name\n}\n", `const.string "km" unit-metadata=<km>.Name`},
		{"fn Read() bool {\n    return <m>.BaseUnit\n}\n", `const.bool true unit-metadata=<m>.BaseUnit`},
		{"fn Read() bool {\n    return <km>.BaseUnit\n}\n", `const.bool false unit-metadata=<km>.BaseUnit`},
		{"fn Read() UnitKind {\n    return <m>.Kind\n}\n", "enum.constant case=#2 unit-metadata=<m>.Kind"},
		{"fn Read() UnitKind {\n    return <minute>.Kind\n}\n", "enum.constant case=#0 unit-metadata=<minute>.Kind"},
		{"fn Read() Domain {\n    return <flow>.Kind\n}\n", "enum.constant case=#1 unit-metadata=<flow>.Kind"},
		{"fn Read() UnitCategory {\n    return <flow>.Category\n}\n", "enum.constant case=#4 unit-metadata=<flow>.Category"},
		{"fn Read() UnitTransform {\n    return <bel>.Transform\n}\n", "enum.constant case=#2 unit-metadata=<bel>.Transform"},
		{"fn Read() UnitStatus {\n    return <m>.Status\n}\n", "enum.constant case=#0 unit-metadata=<m>.Status"},
		{"fn Read() decimal {\n    return <km>.Scale\n}\n", `"1000" coefficient=1000 scale=0 unit-metadata=<km>.Scale exact=1000`},
		{"fn Read() decimal {\n    return <third>.Scale\n}\n", `coefficient=3333333333333333333 scale=19 unit-metadata=<third>.Scale exact=1/3 projected`},
		{"fn Read() decimal {\n    return <giga>.Scale\n}\n", `coefficient=1000000000000000000 scale=-12 unit-metadata=<giga>.Scale exact=1000000000000000000000000000000 :`},
		{"fn Read() decimal {\n    return <celsius>.Offset\n}\n", `coefficient=27315 scale=2 unit-metadata=<celsius>.Offset exact=5463/20`},
		{"fn Read() decimal {\n    return <bel>.Reference\n}\n", "unit-metadata=<bel>.Reference exact=1"},
		{"fn Read() uint {\n    return <minute>.Dimension.AxisCount\n}\n", "const.int 1 unit-metadata=<minute>.Dimension.AxisCount"},
		{"fn Read() bool {\n    return <bel>.Dimension.IsDimensionless\n}\n", "const.bool true unit-metadata=<bel>.Dimension.IsDimensionless"},
		{"fn Read() bool {\n    return <m>.Dimension == <km>.Dimension\n}\n", "const.bool true unit-metadata=<m>.Dimension==<km>.Dimension"},
		{"fn Read() bool {\n    return <m>.Dimension != <minute>.Dimension\n}\n", "const.bool true unit-metadata=<m>.Dimension!=<minute>.Dimension"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			module, err := analyzedModule(t, unitMetadataIRUnits+tc.body, 15)
			if err != nil {
				t.Fatal(err)
			}
			if printed := Format(module); !strings.Contains(printed, tc.want) {
				t.Fatalf("want %q in\n%s", tc.want, printed)
			}
		})
	}
}

// A UnitDimension value that escapes compile-time folding, and a reference
// quantity read, have no Semantic IR representation yet and fail closed.
//
// Rules:
//   - rules/types/units.md — "Semantic IR and lowering" (MD-062)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 12(8), 15(4)
func TestUnitDimensionValuesFailClosed(t *testing.T) {
	for _, body := range []string{
		"fn Read() UnitDimension {\n    return <m>.Dimension\n}\n",
		"fn Read() uint {\n    let dimension := <m>.Dimension\n    return dimension.AxisCount\n}\n",
		"fn Same(left: UnitDimension, right: UnitDimension) bool {\n    return left == right\n}\n",
		"fn Same(left: UnitDimension) bool {\n    return left == <m>.Dimension\n}\n",
	} {
		t.Run(body, func(t *testing.T) {
			_, err := analyzedModule(t, unitMetadataIRUnits+body, 15)
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an unsupported-feature error, got %v", err)
			}
		})
	}
}

// Provenance is verified: it annotates only constants, and a decimal is
// projected only for Scale and only when it differs from the exact value.
// Rules: rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 9(7)–9(8), 12(7).
func TestUnitMetadataProvenanceVerification(t *testing.T) {
	decimal, err := parseDecimal("0.5")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		op    Operation
		valid bool
	}{
		{Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1/2"}}, true},
		{Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1/3", Projected: true}}, true},
		{Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1/3"}}, false},
		{Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Offset", Exact: "1/3", Projected: true}}, false},
		{Operation{Kind: OpConstDecimal, Decimal: &decimal, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1/2", Projected: true}}, false},
		{Operation{Kind: OpConstDecimal, Decimal: &DecimalConstant{Coefficient: big.NewInt(1), Scale: -30, Lexeme: "1e30"}, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1000000000000000000000000000000"}}, true},
		{Operation{Kind: OpConstDecimal, Decimal: &DecimalConstant{Coefficient: big.NewInt(1), Scale: -29, Lexeme: "1e29"}, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Scale", Exact: "1000000000000000000000000000000"}}, false},
		{Operation{Kind: OpConstString, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Name", Exact: "1"}}, false},
		{Operation{Kind: OpStorageLoad, UnitMetadata: &UnitMetadataFacts{Unit: "u", Member: "Name"}}, false},
	} {
		if err := verifyUnitMetadataOperation(tc.op); (err == nil) != tc.valid {
			t.Errorf("%s %+v: valid=%v, got %v", tc.op.Kind, *tc.op.UnitMetadata, tc.valid, err)
		}
	}
}
