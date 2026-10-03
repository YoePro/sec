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
impl A {
    LongName: "Ampere"
    Symbol: "A"
    BaseUnit: true
    Status: active
    Dimension: [electric_current^1]
    Scale: 1
    System: SI
}
unit K physical
impl K {
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

// Within one module a unit and a type of the same spelling remain a
// declaration conflict, because which one `impl Name` targets is undecided
// (missing-decisions.yaml MD-040).
func TestUnitAndSameSpelledTypeInOneModuleConflict(t *testing.T) {
	for _, source := range []string{
		"module main\nunit Volt physical\ntype Volt int\n",
		"module main\ntype Volt int\nunit Volt physical\n",
	} {
		errors := NewAnalyzer().Analyze(parser.New(lexer.New(source)).ParseProgram())
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "conflicts with") {
			t.Fatalf("errors for %q = %+v", source, errors)
		}
	}
}
