package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// MD-049 §§ 3-5, 10-11, 17: inferred and fixed scales, coefficient-capacity
// reduction and grid defaults, asserted as exact (coefficient, scale) pairs.
func TestDecimalScaleInferenceAndDefaults(t *testing.T) {
	cases := []struct {
		id, source, name string
		scale            string // resolved scale, or "variable"
		pair             string // implicit default (c,s)
		inferred         bool   // A2012 expected on the type
	}{
		{"T01", "type Negative decimal range -1.00..<0.000", "Negative", "3", "(-1,3)", true},
		{"T02", "type Pos decimal range 0.00..", "Pos", "2", "(0,2)", true},
		{"T03", "type Neg decimal range ..<0.000", "Neg", "3", "(-1,3)", true},
		{"T04", "type Wide decimal range 0.001..1000000000000000000", "Wide", "0", "(1,0)", true},
		{"T05", "type Wide128 decimal128 range 0.001..1000000000000000000", "Wide128", "3", "(1,3)", true},
		{"T08", "type Window decimal scale 2 range 0.001..<0.020", "Window", "2", "(1,2)", false},
		{"T09", "type Steps decimal scale -2 range ..<0", "Steps", "-2", "(-1,-2)", false},
		{"T10", "type Money decimal scale 2", "Money", "2", "(0,2)", false},
		// T11 (`range ..`) is a parser error by owner decision 2026-11-22
		// (contracts.md § 16(6)); a bound-free decimal type stays variable-scale.
		{"T11 variant", "type Broad decimal", "Broad", "variable", "", false},
		{"T12", "type RangeZero decimal range 0.000..0.000", "RangeZero", "3", "(0,3)", true},
		{"T13", "type Multi decimal range -100.00..100.00 range -10.000..10.000", "Multi", "3", "(0,3)", true},
		{"T14", "type Base decimal scale 2\ntype Price Base range 1..10", "Price", "2", "(100,2)", false},
		{"T18", "type Root decimal\ntype Child Root range -1.00..<0.000", "Child", "3", "(-1,3)", true},
		{"T19", "type Base decimal range -1.00..<0.00\ntype Child Base range -1.000..<0.000", "Child", "2", "(-1,2)", false},
		{"T27", "type Pos decimal scale (1 + 2) range 0..1", "Pos", "3", "(0,3)", false},
		{"T54", "type Steps decimal scale (0 - 3) range ..<0", "Steps", "-3", "(-1,-3)", false},
		{"T55", "type Open decimal scale 2 range 0..", "Open", "2", "(0,2)", false},
		{"T56", "type Edge decimal scale 2 range 0.001..0.019", "Edge", "2", "(1,2)", false},
		{"T58", "type Edge decimal scale 2 range -0.019..-0.001", "Edge", "2", "(-1,2)", false},
		{"T60", "type Edge decimal scale 2 range 0.001..<0.021", "Edge", "2", "(1,2)", false},
		{"T61", "type SignedMin decimal scale 0 range -9223372036854775808..0", "SignedMin", "0", "(0,0)", false},
		{"T62", "type SignedMax decimal scale 0 range 0..9223372036854775807", "SignedMax", "0", "(0,0)", false},
		{"T64", "type NegativeCapacity decimal range -9223372036854775808..0.000", "NegativeCapacity", "0", "(0,0)", true},
		{"T65", "type Huge decimal range 0.001..10000000000000000000", "Huge", "-1", "(1,-1)", true},
	}
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			a, errs := analyzeScaleTypes(t, "module main\n\n"+test.source+"\n")
			if len(errs) != 0 {
				t.Fatalf("errors: %v", errs)
			}
			typ := a.types[test.name]
			scale, fixed := FixedDecimalScaleOf(typ)
			got := "variable"
			if fixed {
				got = itoa32(scale)
			}
			if got != test.scale {
				t.Fatalf("scale = %s, want %s", got, test.scale)
			}
			if test.pair != "" {
				resolution := defaultValueOf(typ, map[string]bool{})
				if pair := decimalPair(resolution.Value.Decimal); pair != test.pair {
					t.Fatalf("default = %s (%s), want %s", pair, resolution.Kind, test.pair)
				}
			}
			inferred := false
			for _, warning := range a.Warnings() {
				if warning.ID == diagnostics.InferredDecimalScale && strings.HasPrefix(warning.Message, test.name+" infers") {
					inferred = true
				}
			}
			if inferred != test.inferred {
				t.Fatalf("A2012 on %s = %v, want %v (%v)", test.name, inferred, test.inferred, a.Warnings())
			}
		})
	}
}

// MD-049 § 17(1): invalid fixed-scale declarations and explicit defaults.
func TestDecimalScaleInferenceRejections(t *testing.T) {
	cases := []struct{ id, source, wantID, message string }{
		{"T06", "type WideBad decimal scale 3 range 0.001..1000000000000000000", diagnostics.UnsatisfiableContractSet, "does not fit a 64-bit coefficient at scale 3"},
		{"T07", "type Window decimal scale 2 range 0.001..<0.009", diagnostics.UnsatisfiableContractSet, "no value of Window at scale 2"},
		{"T20", "type Base decimal scale 2 range 0..10\ntype Bad Base range 11..12", diagnostics.UnsatisfiableContractSet, "no value of Bad at scale 2"},
		{"T24", "type Amount decimal scale 2 default 0.001", diagnostics.DefaultNotRepresentable, "not representable by Amount at scale 2"},
		{"T25", "type Amount decimal scale 2 range 0..10 default 11.00", diagnostics.DefaultViolatesContract, "default value 11.00 is invalid"},
		{"T57", "type Edge decimal scale 2 range 0.001..<0.010", diagnostics.UnsatisfiableContractSet, "no value of Edge at scale 2"},
		{"T59", "type Edge decimal scale 2 range -0.019..<-0.010", diagnostics.UnsatisfiableContractSet, "no value of Edge at scale 2"},
		{"T63", "type TooHigh decimal scale 0 range 0..9223372036854775810", diagnostics.UnsatisfiableContractSet, "does not fit a 64-bit coefficient at scale 0"},
	}
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			_, errs := analyzeScaleTypes(t, "module main\n\n"+test.source+"\n")
			for _, err := range errs {
				if err.ID == test.wantID && strings.Contains(err.Message, test.message) {
					return
				}
			}
			t.Fatalf("want %s %q, got %v", test.wantID, test.message, errs)
		})
	}
}

// An explicit default is rescaled exactly onto the fixed grid (§ 65A(3)).
func TestFixedScaleExplicitDefaultIsRescaled(t *testing.T) {
	a, errs := analyzeScaleTypes(t, "module main\n\ntype Amount decimal scale 2 range 0..10 default 1.5\n")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if got := decimalPair(a.types["Amount"].ExplicitDefault.Decimal); got != "(150,2)" {
		t.Fatalf("explicit default = %s", got)
	}
}

// MD-049 § 15(3), T37-T40: a constant conversion into a fixed-scale type is
// placed exactly on its grid, otherwise rejected at compile time with the
// error matrix order: rounding (PrecisionLoss), coefficient width
// (OutOfRange) before the range contract, then the contract.
func TestFixedScaleConstantConversions(t *testing.T) {
	prefix := "module main\n\ntype Amount decimal scale 2 range 0..1000\n\nfn F() Amount {\n    return "
	a, errs := analyzeScaleTypes(t, prefix+"Amount(12.340m)\n}\n")
	if len(errs) != 0 {
		t.Fatalf("T37: %v", errs)
	}
	found := false
	for _, pair := range a.decimalConstantConversions {
		found = found || decimalPair(&pair) == "(1234,2)"
	}
	if !found {
		t.Fatalf("T37 result = %v", a.decimalConstantConversions)
	}
	for _, test := range []struct{ id, argument, message string }{
		{"T38", "Amount(12.345m)", "ConversionError.PrecisionLoss"},
		{"T39", "Amount(1000.01m)", "violates range 0..1000"},
		{"T40", "Amount(1000000000000000000m)", "ConversionError.OutOfRange"},
	} {
		_, errs := analyzeScaleTypes(t, prefix+test.argument+"\n}\n")
		if len(errs) != 1 || errs[0].ID != diagnostics.ValueViolatesContract || !strings.Contains(errs[0].Message, test.message) {
			t.Fatalf("%s: %v", test.id, errs)
		}
	}
	// A runtime value is left to the runtime conversion.
	if _, errs := analyzeScaleTypes(t, "module main\n\ntype Amount decimal scale 2 range 0..1000\n\nfn F(value: decimal) Amount {\n    return try Amount(value) {\n        Err(_) => Amount(0m)\n    }\n}\n"); len(errs) != 0 {
		t.Fatalf("runtime conversion: %v", errs)
	}
}
