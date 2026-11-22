package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeScaleTypes(t *testing.T, source string) (*Analyzer, []Error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "scale.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	return a, a.Analyze(result.Program)
}

// MD-049 contracts.md § 16(1), types.md § 45A(2): the fixed scale is recorded
// locally or inherited through named generations, an identical restatement
// is valid, and computed or negative compile-time integers are accepted
// (T10, T14-T15, T26-T27, T54-T55).
func TestDecimalScaleContractRecordsFixedScale(t *testing.T) {
	a, errs := analyzeScaleTypes(t, `module main

let scale := 3

type Money decimal scale 2
type Price Money range 1..10
type Restated Money scale 2
type Wide decimal128 scale (1 + 2) range 0..1
type Steps decimal scale (0 - 3) range ..<0
type Open decimal scale 2 range 0..
type Plain decimal
`)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	for name, want := range map[string]int32{"Money": 2, "Price": 2, "Restated": 2, "Wide": 3, "Steps": -3, "Open": 2} {
		got, ok := FixedDecimalScaleOf(a.types[name])
		if !ok || got != want {
			t.Fatalf("%s scale = %d, %v; want %d", name, got, ok, want)
		}
	}
	if _, ok := FixedDecimalScaleOf(a.types["Plain"]); ok {
		t.Fatal("a plain named decimal must stay variable-scale")
	}
}

// MD-049 § 15(2): each invalid scale contract has its registered error
// (T16-T17, T21-T23, T28).
func TestDecimalScaleContractRejections(t *testing.T) {
	cases := []struct{ name, source, id, message string }{
		{"T16 conflicting inherited scale", "type Base decimal scale 2\ntype Bad Base scale 3", diagnostics.UnsatisfiableContractSet, "conflicts with the inherited fixed scale 2"},
		{"T17 duplicate local scale", "type Bad decimal scale 2 scale 2", diagnostics.UnsatisfiableContractSet, "more than one scale contract"},
		{"T21 non-decimal base", "type NotDecimal int scale 2", diagnostics.InapplicableContract, "scale contract does not apply to int"},
		{"T22 outside int32", "type Bad decimal scale 2147483648", diagnostics.InvalidContractArgument, "outside the signed int32 range"},
		{"T23 decimal argument", "type Bad decimal scale 1.5", diagnostics.InvalidContractArgument, "must be a compile-time integer"},
		{"T28 runtime argument", "fn RuntimeScale() int {\n    let now := datetime.Now\n    discard now\n    return 2\n}\ntype Bad decimal scale RuntimeScale()", "", "scale value RuntimeScale() is not compile-time evaluable"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, errs := analyzeScaleTypes(t, "module main\n\n"+test.source+"\n")
			for _, err := range errs {
				// T28 accepts the required-CTE diagnostic or S1066 (§ 17(1)).
				if (test.id == "" || err.ID == test.id) && strings.Contains(err.Message, test.message) {
					return
				}
			}
			t.Fatalf("want %s %q, got %v", test.id, test.message, errs)
		})
	}
}
