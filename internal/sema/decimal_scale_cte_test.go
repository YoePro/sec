package sema

import (
	"testing"

	"sec/internal/ast"
)

func decimalPair(value *DecimalValue) string {
	if value == nil || value.Coefficient == nil {
		return "nil"
	}
	return "(" + value.Coefficient.String() + "," + itoa32(value.Scale) + ")"
}

func itoa32(value int32) string {
	if value < 0 {
		return "-" + itoa32(-value)
	}
	if value < 10 {
		return string(rune('0' + value))
	}
	return itoa32(value/10) + string(rune('0'+value%10))
}

// MD-049 § 13(1)-(2), semantic_ir.md § 128(1): compile-time decimals keep a
// (coefficient, scale) pair through literals, arithmetic with the runtime
// scale policy (including the zero-product max), negation, integers at scale
// 0 and immutable bindings; equal rationals keep different scales (T29-T36).
func TestCompileTimeDecimalsCarryScale(t *testing.T) {
	a, errs := analyzeScaleTypes(t, `module main

let T29a := 1.20m
let T29b := 1.200m
let T30 := 1.20m + 2.3m
let T31 := 1.20m * 2.0m
let T32 := 0.00m * 0.000m
let T33 := 0.000m * 0.00m
let T34 := 0.000m * 0.000m
let T35 := 0.00m * 1.234m
let T36 := 0.00m + 0.000m
let Negative := -1.00m
let Mixed := 2 * 1.5m
let Chained := T31 + 0.0001m
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	a.currentModule = "main"
	for name, want := range map[string]string{
		"T29a": "(120,2)", "T29b": "(1200,3)", "T30": "(350,2)", "T31": "(2400,3)", "T32": "(0,3)", "T33": "(0,3)",
		"T34": "(0,3)", "T35": "(0,3)", "T36": "(0,3)", "Negative": "(-100,2)", "Mixed": "(30,1)", "Chained": "(24001,4)",
	} {
		value, ok := a.semanticCompileTimeDecimal(&ast.Identifier{Value: name})
		if !ok || decimalPair(&value) != want {
			t.Fatalf("%s = %s (%v), want %s", name, decimalPair(&value), ok, want)
		}
	}
}

// MD-049 §§ 3(1), 3(4), 13(4): every present range bound of a decimal type
// keeps its own scale-bearing value: literal spelling, integer scale 0 and a
// computed bound's represented scale (T01, T48); integer bases keep none.
func TestDecimalRangeBoundsCarryScale(t *testing.T) {
	a, errs := analyzeScaleTypes(t, `module main

type Negative decimal range -1.00..<0.000
type Mixed decimal range 0.00..100
type Computed decimal range (1.20m * 2.0m)..10m
type Open decimal range ..<0.000
type Whole int range 0..10
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	bounds := func(name string) string {
		for _, contract := range a.types[name].Contracts {
			if r, ok := contract.(RangeContract); ok {
				return decimalPair(r.MinDecimal) + ".." + decimalPair(r.MaxDecimal)
			}
		}
		return "no range"
	}
	for name, want := range map[string]string{
		"Negative": "(-100,2)..(0,3)", "Mixed": "(0,2)..(100,0)", "Computed": "(2400,3)..(10,0)", "Open": "nil..(0,3)", "Whole": "nil..nil",
	} {
		if got := bounds(name); got != want {
			t.Fatalf("%s bounds = %s, want %s", name, got, want)
		}
	}
}

// compile_time_evaluation.md § 2(3), operators.md § 27.2: an inexact decimal
// quotient rounds once with nearest-even at the finest scale that fits, the
// same as at run time; division by zero stays an error.
func TestCompileTimeDecimalQuotientRoundsLikeRuntime(t *testing.T) {
	a, errs := analyzeScaleTypes(t, `module main

let One: decimal := 1m
let Three: decimal := 3m

type Third decimal default One / Three
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if got := decimalPair(a.types["Third"].ExplicitDefault.Decimal); got != "(3333333333333333333,19)" {
		t.Fatalf("One / Three default = %s", got)
	}
}
