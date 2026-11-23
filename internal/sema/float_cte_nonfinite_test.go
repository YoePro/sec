package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// P20 §§ 28, 68, 77, 82: semantic CTE folds float type properties and
// non-finite IEEE results: overflow is +Inf, Inf - Inf is NaN, negation of
// an infinity flips its sign, NaN is unordered and Min is the most negative
// finite value.
func TestCompileTimeNonFiniteFloats(t *testing.T) {
	source := `module main

let Max: float64 := float64.Max
let Min: float32 := float32.Min
let Epsilon: float32 := float32.Epsilon
let Infinity: float64 := float64.Infinity
let Overflow: float64 := Max * 2.0
let Negated: float64 := -Infinity
let Invalid: float64 := Infinity - Infinity
let SelfEqual: bool := Invalid == Invalid
let SelfDifferent: bool := Invalid != Invalid
let InfinityAbove: bool := Infinity > Max

fn main() int {
    return 0
}
`
	p := parser.New(lexer.NewWithFile(source, "nonfinite.sec"))
	program := p.ParseProgram()
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	want := map[string]string{
		"Max": "1.7976931348623157e+308", "Min": "-3.4028235e+38", "Epsilon": "1.1920929e-07",
		"Infinity": "+Inf", "Overflow": "+Inf", "Negated": "-Inf", "Invalid": "NaN",
		"SelfEqual": "false", "SelfDifferent": "true", "InfinityAbove": "true",
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
