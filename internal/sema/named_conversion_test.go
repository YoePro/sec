package sema

import (
	"testing"

	"sec/internal/ast"
)

// A named integer keeps its identity: an explicit conversion over one carrier
// records a named-conversion plan (a constant shapes into the carrier), and
// same-type operators resolve with the named operand type. A unit-bearing or
// contract-bearing named type, or a carrier change, records no plan.
//
// Rules: rules/types/types.md — Named types; rules/foundations/operators.md —
// "Named types", "Contracts"; rules/compiler/semantic_ir.md — §§ 7(4), 59(2).
func TestNamedIntegerConversionAndOperatorPlans(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
type Meters int
type Kilometers Meters
type Count uint8
type Percent int range 0..100

fn Use(raw: int, wide: int64, distance: Meters) bool {
    let a := Meters(raw)
    let b := int(distance)
    let c := Kilometers(distance)
    let d := Count(4)
    let e := try Percent(raw) {
        Err(_) => Percent(0)
    }
    let f := Meters(int(wide))
    discard b
    discard c
    discard d
    discard e
    return a + distance < f
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	plans := map[string]ResolvedNamedConversion{}
	for call, plan := range analyzer.resolvedNamedConversions {
		plans[call.String()] = plan
	}
	for source, target := range map[string]string{"Meters(raw)": "Meters", "int(distance)": "int", "Kilometers(distance)": "Kilometers", "Count(4)": "Count", "Meters(int(wide))": "Meters"} {
		if plan, ok := plans[source]; !ok || plan.TargetType.Name != target {
			t.Errorf("%s: plan %+v recorded=%v, want target %s", source, plan, ok, target)
		}
	}
	if plan := plans["Count(4)"]; plan.SourceType.Name != "uint8" {
		t.Errorf("Count(4) source = %s, want the uint8 carrier", plan.SourceType.Name)
	}
	if _, ok := plans["Percent(raw)"]; ok {
		t.Error("a contract-bearing named type must keep its validated conversion")
	}
	kinds := map[ResolvedOperatorKind]string{}
	for expr, resolved := range analyzer.resolvedOperators {
		if _, infix := expr.(*ast.InfixExpression); infix {
			kinds[resolved.Kind] = resolved.LeftType.Name
		}
	}
	if kinds[ResolvedIntegerAddChecked] != "Meters" || kinds[ResolvedIntegerCompareLT] != "Meters" {
		t.Fatalf("named operators = %v, want add and compare over Meters", kinds)
	}
}
