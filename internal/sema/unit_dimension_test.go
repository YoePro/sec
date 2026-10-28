package sema

import (
	"strings"
	"testing"
)

const unitDimensionSource = `module main

unit m physical
impl <m> {
    Dimension: [length^1]
}

unit km physical
impl <km> {
    Dimension: [length^1]
    Scale: 1000
}

unit mps physical
impl <mps> {
    Dimension: [length^1, time^-1]
}

unit pace physical
impl <pace> {
    Dimension: [time^-1, length^1]
}

unit J physical
impl <J> {
    Dimension: [mass^1, length^2, time^-2]
    Kind: UnitKind.Energy
}

unit Nm physical
impl <Nm> {
    Dimension: [mass^1, length^2, time^-2]
    Kind: UnitKind.Torque
}

unit dB ratio

unit packet other
unit batch other
unit PacketsPerBatch other
impl <PacketsPerBatch> {
    Dimension: [packet^1, batch^-1]
}

type Measured struct {
    dimension: UnitDimension,
}

fn SameDimension(left: UnitDimension, right: UnitDimension) bool {
    return left == right
}
`

// UnitDimension is an opaque, copyable value with exactly AxisCount,
// IsDimensionless, == and !=. Metadata reads produce canonical dimensions:
// order-independent, equal for energy and torque without equal Kinds, empty
// for ratio units, and equal to a structural quotient of domain units.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (7), (8)
//   - rules/library/core-library.md — § 27.5
//   - rules/foundations/operators.md — "Unit dimension equality"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(2)–7(7), 8(3), 13(6)
func TestUnitDimensionValues(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, unitDimensionSource+`
fn Use() void {
    let length: UnitDimension := <m>.Dimension
    let copy := length
    let same: bool := length == <km>.Dimension
    let ordered: bool := <mps>.Dimension == <pace>.Dimension
    let differ: bool := <m>.Dimension != <mps>.Dimension
    let count: uint := <mps>.Dimension.AxisCount
    let empty: bool := <dB>.Dimension.IsDimensionless
    let energy: bool := SameDimension(<J>.Dimension, <Nm>.Dimension)
    let measured := Measured{ dimension: <m>.Dimension }
    let stored := measured.dimension == copy
}
`)
	assertSemaErrors(t, errors, nil)
	units := analyzer.units
	if !units["mps"].Dimension.Equal(units["pace"].Dimension) || units["m"].Dimension.Equal(units["mps"].Dimension) {
		t.Fatalf("order-independent dimensions: mps=%v pace=%v", units["mps"].Dimension, units["pace"].Dimension)
	}
	if !units["J"].Dimension.Equal(units["Nm"].Dimension) || units["J"].Kind == units["Nm"].Kind {
		t.Fatalf("energy/torque: %+v %+v", units["J"], units["Nm"])
	}
	if !units["dB"].Dimension.IsZero() || !units["dB"].DimensionEstablished {
		t.Fatalf("ratio dimension = %+v", units["dB"])
	}
	quotient := units["packet"].Dimension.Div(units["batch"].Dimension)
	if !units["PacketsPerBatch"].Dimension.Equal(quotient) {
		t.Fatalf("declared domain axes differ from unit axes: %v vs %v", units["PacketsPerBatch"].Dimension, quotient)
	}
	if !TriviallyCopyable(analyzer.types["UnitDimension"]) || IsDefaultable(analyzer.types["UnitDimension"]) || !EqualityComparable(analyzer.types["UnitDimension"]) {
		t.Fatal("UnitDimension must be copyable, comparable and not defaultable")
	}
}

// UnitDimension has no constructor, default, setter, index, iteration,
// ordering, arithmetic or other members.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (7)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(3)–7(5), 14(6)
func TestUnitDimensionRejectsUnsupportedOperations(t *testing.T) {
	for _, body := range []string{
		"let less := <m>.Dimension < <km>.Dimension",
		"let sum := <m>.Dimension + <km>.Dimension",
		"let product := <m>.Dimension * <km>.Dimension",
		"let number := <m>.Dimension == 1",
		"let d: UnitDimension",
		"let d := UnitDimension()",
		"let d := <m>.Dimension\n    d.AxisCount = 3",
		"let axes := <m>.Dimension.Axes",
		"let axis := <m>.Dimension[0]",
		"for axis in <m>.Dimension {\n    }",
		"let kind := <m>.Dimension == UnitKind.Length",
	} {
		t.Run(body, func(t *testing.T) {
			errors := analyzeSource(t, unitDimensionSource+"fn Use() void {\n    "+body+"\n}\n")
			if len(errors) == 0 {
				t.Fatal("accepted")
			}
		})
	}
}

// Domain axes belong to their defining module: equal spellings in different
// modules are distinct axes, a qualified axis names the imported module's
// axis and must exist there, two spellings of one axis are a duplicate, and
// oversized exponents are rejected.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types" (7)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(9)–7(11), 13(7), 14(4)
func TestUnitDimensionAxesAreModuleScoped(t *testing.T) {
	first := `module alpha
unit Flow physical
impl <Flow> {
    Dimension: [flux^1, time^-1]
}
`
	second := `module beta
import q "lab/alpha"
unit Stream physical
impl <Stream> {
    Dimension: [flux^1, time^-1]
}
unit Imported physical
impl <Imported> {
    Dimension: [q.flux^1, time^-1]
}
`
	program := unitNamespaceProgram(first, "alpha.sec", second, "beta.sec")
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	units := analyzer.units
	if units["Flow"].Dimension.Equal(units["Stream"].Dimension) {
		t.Fatalf("same spelling in two modules compared equal: %v", units["Flow"].Dimension)
	}
	if !units["Flow"].Dimension.Equal(units["Imported"].Dimension) || units["Flow"].Dimension.Base["alpha::flux"] != 1 {
		t.Fatalf("aliased axis differs: %v vs %v", units["Flow"].Dimension, units["Imported"].Dimension)
	}
	for _, tc := range []struct{ dimension, want string }{
		{"[q.missing^1]", "unknown dimension axis q.missing for unit Bad: no unit metadata of module alpha declares axis missing"},
		{"[flux^1, beta.flux^1]", "duplicate dimension axis for unit Bad: beta.flux and flux name the same axis"},
		{"[length^40000]", "invalid dimension metadata for unit Bad"},
		{"[length^0]", "invalid dimension metadata for unit Bad"},
	} {
		t.Run(tc.dimension, func(t *testing.T) {
			bad := second + "unit Bad physical\nimpl <Bad> {\n    Dimension: " + tc.dimension + "\n}\n"
			errors := NewAnalyzer().Analyze(unitNamespaceProgram(first, "alpha.sec", bad, "beta.sec"))
			if len(errors) != 1 || !strings.Contains(errors[0].Message, tc.want) {
				t.Fatalf("want exactly %q, got %+v", tc.want, errors)
			}
		})
	}
}
