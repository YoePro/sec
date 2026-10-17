package sema

import (
	"testing"

	"sec/internal/ast"
)

// An explicit built-in integer conversion records Sema's plan: exact when the
// source domain fits the target on every supported width or the argument is a
// validated compile-time constant, checked otherwise (types.md "Explicit
// conversions"; MD-104). char and rune comparisons resolve to character
// comparisons, never to integer ones (operators.md "Character ordering",
// "Rune ordering"; MD-105).
func TestIntegerConversionPlansAndCharacterComparisons(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
fn Conversions(small: int8, unsigned: uint8, wide: int) int8 {
    let a := int(small)
    let b := int(unsigned)
    let c := 100
    let d := int8(c)
    discard a
    discard b
    discard d
    return int8(wide)
}

fn Compare(r: rune, s: rune, c: char) bool {
    return r < s && c == 'x'
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	exact := map[string]bool{}
	for call, plan := range analyzer.resolvedIntegerConversions {
		exact[call.Arguments[0].String()] = plan.Exact
	}
	for argument, want := range map[string]bool{"small": true, "unsigned": true, "c": true, "wide": false} {
		if got, ok := exact[argument]; !ok || got != want {
			t.Errorf("conversion of %s: exact=%v recorded=%v, want exact=%v", argument, got, ok, want)
		}
	}
	kinds := map[ResolvedOperatorKind]bool{}
	for expr, resolved := range analyzer.resolvedOperators {
		if _, infix := expr.(*ast.InfixExpression); infix {
			kinds[resolved.Kind] = true
		}
	}
	if !kinds[ResolvedCharacterCompareLT] || !kinds[ResolvedCharacterCompareEQ] {
		t.Fatalf("character comparisons not resolved: %v", kinds)
	}
}
