package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const unitNamespaceUnitsModule = `
module units
unit A physical
impl <A> {
    LongName: "Ampere"
    Symbol: "A"
    BaseUnit: true
    Status: active
    Dimension: [electric_current^1]
    Scale: 1
    System: SI
}
unit K physical
impl <K> {
    LongName: "Kelvin"
    Symbol: "K"
    BaseUnit: true
    Status: active
    Dimension: [thermodynamic_temperature^1]
    Scale: 1
    System: SI
}
`

const unitNamespaceApplicationModule = `
module application
type A int
type K struct {
    value: int,
}
impl K {
    fn Double() int {
        return self.value * 2
    }
}
fn Use() int {
    let a: A := A(3)
    let k := K{ value: 2 }
    let current: decimal<A> := 2.5m
    let temperature: decimal<K> := 300m
    let total: decimal<A> := current + current
    return k.Double()
}
`

func unitNamespaceProgram(first string, firstFile string, second string, secondFile string) *ast.Program {
	program := parser.New(lexer.NewWithFile(first, firstFile)).ParseProgram()
	other := parser.New(lexer.NewWithFile(second, secondFile)).ParseProgram()
	program.Statements = append(program.Statements, other.Statements...)
	return program
}

// A unit symbol has its own namespace, so an ordinary type of another module
// may use the same spelling: the unit keeps its impl metadata and unit
// expressions, and the type keeps its own impl, in either declaration order.
//
// Rules:
//   - rules/types/units.md — "Unit names and compiler-known names"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func TestUnitAndSameSpelledTypeOfAnotherModuleCoexist(t *testing.T) {
	orders := map[string]*ast.Program{
		"unit module first": unitNamespaceProgram(unitNamespaceUnitsModule, "units.sec", unitNamespaceApplicationModule, "application.sec"),
		"type module first": unitNamespaceProgram(unitNamespaceApplicationModule, "application.sec", unitNamespaceUnitsModule, "units.sec"),
	}
	for name, program := range orders {
		t.Run(name, func(t *testing.T) {
			analyzer := NewAnalyzer()
			if errors := analyzer.Analyze(program); len(errors) != 0 {
				t.Fatalf("unexpected errors: %+v", errors)
			}
			if typ := analyzer.Types()["K"]; typ.Kind != StructType || typ.Module != "application" {
				t.Fatalf("type K = %+v", typ)
			}
			if unit, ok := analyzer.units["A"]; !ok || unit.Category != PhysicalUnit {
				t.Fatalf("unit A = %+v", unit)
			}
		})
	}
}

// MD-040: a unit is always referenced as <Unit>, so a unit and an ordinary
// type of the same spelling coexist even in one module. `impl <Volt>` holds
// only unit metadata and unit functions, `impl Volt` only the type's members,
// in either declaration order; `<Volt>(value)` converts, `<Volt>.Symbol`
// reads metadata, `<Volt>.Name(...)` calls a unit function, while
// `Volt(value)` and `Volt.SizeOf` keep their ordinary type meaning.
//
// Rules:
//   - rules/types/units.md — "Unit names and compiler-known names"
//   - rules/declarations/impl.md — unit impl blocks
func TestUnitAndSameSpelledTypeInOneModuleCoexist(t *testing.T) {
	unit := `unit Volt physical

impl <Volt> {
    LongName: "Volt"
    Symbol: "V"
    Dimension: [mass^1, length^2, time^-3, electric_current^-1]
    Scale: 1
    System: SI

    fn Volt(millivolts: decimal<mV>) <Volt> {
        return <Volt>(millivolts)
    }

    fn Half(value: decimal<Volt>) decimal<Volt> {
        return value / 2
    }
}

unit mV physical

impl <mV> {
    Dimension: [mass^1, length^2, time^-3, electric_current^-1]
    Scale: 1 / 1000
}
`
	typ := `type Volt uint

impl Volt {
    fn IsZero() bool {
        return self == 0
    }
}
`
	use := `fn Use(value: decimal<Volt>, raw: uint, small: decimal<mV>) bool {
    let converted := <mV>(small)
    let fromMillivolts := <Volt>(small)
    let symbol: string := <Volt>.Symbol
    let name: string := <Volt>.LongName
    let half := <Volt>.Half(value)
    let ordinary := Volt(raw)
    let bytes := Volt.SizeOf
    return ordinary.IsZero()
}
`
	for name, source := range map[string]string{
		"unit first": "module main\n" + unit + typ + use,
		"type first": "module main\n" + typ + unit + use,
	} {
		analyzer := NewAnalyzer()
		if errors := analyzer.Analyze(parser.New(lexer.New(source)).ParseProgram()); len(errors) != 0 {
			t.Fatalf("%s: unexpected errors: %+v", name, errors)
		}
		if typ := analyzer.Types()["Volt"]; typ.Kind != UintType || !typ.Named {
			t.Fatalf("%s: type Volt = %+v", name, typ)
		}
		if unit, ok := analyzer.units["Volt"]; !ok || unit.Symbol != "V" {
			t.Fatalf("%s: unit Volt = %+v", name, unit)
		}
	}
}

// Legacy unqualified unit references are rejected with the <Unit> spelling
// as help: `impl Unit`, a bare unit conversion call and a bare unit type.
// Reading `<rpm>.Kind` is valid since MD-062 (UnitKind.Unknown when absent).
func TestLegacyUnqualifiedUnitReferencesAreRejected(t *testing.T) {
	source := `module main
unit rpm physical

impl rpm {
    Dimension: [time^-1]
    Scale: 1 / 60
}

fn Use(value: decimal<rpm>) void {
    let converted := rpm(value)
    let typed: rpm := value
    let kind := <rpm>.Kind
}
`
	errors := NewAnalyzer().Analyze(parser.New(lexer.New(source)).ParseProgram())
	wants := []string{
		"impl rpm does not name an ordinary type; rpm is a unit",
		"rpm is a unit, not a function or type",
		"rpm is a unit, not a type",
	}
	if len(errors) != len(wants) {
		t.Fatalf("want %d errors, got %+v", len(wants), errors)
	}
	for _, want := range wants {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, want) {
				found = true
				if !strings.Contains(err.Help, "<rpm>") {
					t.Errorf("%q lacks <rpm> help: %q", want, err.Help)
				}
			}
		}
		if !found {
			t.Errorf("missing %q in %+v", want, errors)
		}
	}
}
