package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// P20 §§ 20, 44–55, 59: semantic CTE folds typed primitive conversions that
// Sema proved exact. Widening and exact narrowing keep the value, infinities,
// NaN and the sign of zero; exact float-to-integer yields the integer; integer
// to float is exact; float-to-bool is `une 0`, so NaN is true. Planless
// platform `float` sources (an unsuffixed-width literal) are not folded.
func TestCompileTimeExactNumericConversions(t *testing.T) {
	source := `module main

let Half: float64 := 0.5g
let Seven: int32 := 7
let Four: float64 := 4.0g
let NegativeZero: float64 := -0.0g
let Widen: float64 := float64(float32.Epsilon)
let Narrow: float32 := float32(Half)
let FromInt: float64 := float64(Seven)
let ToInt: int32 := int32(Four)
let InfNarrow: float32 := float32(float64.Infinity)
let NaNNarrow: float32 := float32(float64.NaN)
let NegZero: float32 := float32(NegativeZero)
let NaNTruth: bool := bool(float64.NaN)
let ZeroTruth: bool := bool(NegativeZero)

fn main() int {
    return 0
}
`
	p := parser.New(lexer.NewWithFile(source, "conversions.sec"))
	program := p.ParseProgram()
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	want := map[string]string{
		"Widen": "1.1920928955078125e-07", "Narrow": "0.5", "FromInt": "7", "ToInt": "4",
		"InfNarrow": "+Inf", "NaNNarrow": "NaN", "NegZero": "-0",
		"NaNTruth": "true", "ZeroTruth": "false",
	}
	for _, static := range a.ResolvedStaticDeclarations() {
		expected, ok := want[static.Name]
		if !ok {
			continue
		}
		got := "none"
		if static.Initializer.Scalar != nil {
			got = static.Initializer.Scalar.Lexeme
		} else if static.Initializer.Integer != nil {
			got = static.Initializer.Integer.String()
		} else if static.Initializer.Bool != nil {
			got = map[bool]string{true: "true", false: "false"}[*static.Initializer.Bool]
		}
		if got != expected {
			t.Errorf("%s = %s, want %s", static.Name, got, expected)
		}
		delete(want, static.Name)
	}
	if len(want) != 0 {
		t.Fatalf("statics without an initializer: %v", want)
	}
}

// P20 §§ 29–36, 82: semantic CTE folds the total float core methods with the
// runtime float.* semantics (NaN-propagating Min/Max, -0 below +0, signed-zero
// rounding, ties to even, binary32 classification at 2^-126).
func TestCompileTimeFloatCoreMethods(t *testing.T) {
	source := `module main

let Minus: float64 := -2.5g
let NegativeZero: float64 := -0.0g
let Tiny: float32 := float32.Epsilon * float32.Epsilon * float32.Epsilon * float32.Epsilon * float32.Epsilon * float32.Epsilon
let Abs: float64 := Minus.Abs()
let Round: float64 := Minus.Round()
let Floor: float64 := Minus.Floor()
let Ceiling: float64 := Minus.Ceiling()
let Truncate: float64 := Minus.Truncate()
let NegativeHalf: float64 := -0.5g
let ZeroCeiling: float64 := NegativeHalf.Ceiling()
let MinZero: float64 := NegativeZero.Min(0.0g)
let MaxNaN: float64 := Minus.Max(float64.NaN)
let IsNaN: bool := float64.NaN.IsNaN()
let Subnormal: bool := Tiny.IsSubnormal()
let Normal: bool := Tiny.IsNormal()
let Infinite: bool := float64.Infinity.IsInfinite()
let Finite: bool := float64.Infinity.IsFinite()

fn main() int {
    return 0
}
`
	p := parser.New(lexer.NewWithFile(source, "core.sec"))
	program := p.ParseProgram()
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	want := map[string]string{
		"Abs": "2.5", "Round": "-2", "Floor": "-3", "Ceiling": "-2", "Truncate": "-2",
		"ZeroCeiling": "-0", "MinZero": "-0", "MaxNaN": "NaN", "IsNaN": "true",
		"Subnormal": "true", "Normal": "false", "Infinite": "true", "Finite": "false",
	}
	for _, static := range a.ResolvedStaticDeclarations() {
		expected, ok := want[static.Name]
		if !ok {
			continue
		}
		got := "none"
		if static.Initializer.Scalar != nil {
			got = static.Initializer.Scalar.Lexeme
		} else if static.Initializer.Bool != nil {
			got = map[bool]string{true: "true", false: "false"}[*static.Initializer.Bool]
		}
		if got != expected {
			t.Errorf("%s = %s, want %s", static.Name, got, expected)
		}
		delete(want, static.Name)
	}
	if len(want) != 0 {
		t.Fatalf("statics without an initializer: %v", want)
	}
}
