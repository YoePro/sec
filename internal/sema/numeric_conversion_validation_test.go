package sema

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Typed finite conversion failures obey range-before-precision, including
// half-open signed/unsigned bounds and integers larger than host machine words.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§47–54.
func TestFiniteNumericConversionValidation(t *testing.T) {
	for _, bits := range []int{32, 64} {
		target := Type{Kind: FloatType, FloatBits: bits}
		precision := uint(24)
		if bits == 64 {
			precision = 53
		}
		for _, shift := range []uint{precision, precision + 1, 64, 127, 200, 255} {
			for _, negative := range []bool{false, true} {
				value := new(big.Int).Lsh(big.NewInt(1), shift)
				if negative {
					value.Neg(value)
				}
				for _, delta := range []int64{-1, 0, 1} {
					integer := new(big.Int).Add(value, big.NewInt(delta))
					rat := new(big.Rat).SetInt(integer)
					got := finiteNumericConversionFailure(target, rat)
					// Independent significand oracle: big.Float reports exact
					// rounding; binary32's exponent domain is checked separately.
					want := ConversionFailureKind("")
					if bits == 32 && new(big.Int).Abs(integer).BitLen() > 128 {
						want = ConversionFailureOutOfRange
					} else if new(big.Float).SetPrec(precision).SetInt(integer).Acc() != big.Exact {
						want = ConversionFailurePrecisionLoss
					}
					if got != want {
						t.Fatalf("f%d %s: %s, expected %s", bits, integer, got, want)
					}
				}
			}
		}
	}
	for _, bits := range []uint16{8, 16, 32, 64, 128, 256} {
		for _, signed := range []bool{false, true} {
			target := targetUnsignedIntegerType("uint", bits)
			if signed {
				target = targetSignedIntegerType("int", bits)
			}
			lower := new(big.Rat).SetInt(target.MinInteger)
			upper := new(big.Rat).SetInt(new(big.Int).Add(target.MaxInteger, big.NewInt(1)))
			for _, test := range []struct {
				value   *big.Rat
				failure ConversionFailureKind
			}{
				{lower, ""},
				{new(big.Rat).Sub(lower, big.NewRat(1, 2)), ConversionFailureOutOfRange},
				{new(big.Rat).Add(lower, big.NewRat(1, 2)), ConversionFailurePrecisionLoss},
				{new(big.Rat).Sub(upper, big.NewRat(1, 2)), ConversionFailurePrecisionLoss},
				{upper, ConversionFailureOutOfRange},
			} {
				if got := finiteNumericConversionFailure(target, test.value); got != test.failure {
					t.Fatalf("%s%d %s: %s, expected %s", target.Kind, bits, test.value, got, test.failure)
				}
			}
		}
	}
}

// Source validation proves exact powers/finite maxima, preserves nonfinite
// categories and publishes the guard order without emitting a physical cast.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§44–61.
func TestPrimitiveNumericConversionValidation(t *testing.T) {
	for _, width := range []uint16{0, 32, 64} {
		data, err := os.ReadFile("../../testdata/sema/numeric_conversion_validation.sec")
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		categories := map[string]bool{}
		wideExact := false
		for call := range a.resolvedNumericConversionPlans {
			plan, _ := a.ResolvedNumericConversionPlanOf(call)
			if plan.Proof.FloatCategory != "" {
				categories[plan.Proof.FloatCategory] = true
			}
			if plan.SourceType.Name == "int256" && plan.TargetType.Name == "float64" {
				wideExact = plan.ProvenExact && !plan.RuntimeCheck && plan.Proof.SourceMin.Cmp(plan.Proof.SourceMax) == 0
			}
			switch plan.Kind {
			case NumericConvertFloatToSigned, NumericConvertFloatToUnsigned:
				if !reflect.DeepEqual(plan.Checks, []string{"finite", "range", "integral"}) {
					t.Fatal(plan.Checks)
				}
				if plan.TargetBits != 0 {
					want := new(big.Int).Add(plan.TargetType.MaxInteger, big.NewInt(1))
					if plan.IntegerLowerBound.Cmp(plan.TargetType.MinInteger) != 0 || plan.IntegerUpperExclusive.Cmp(want) != 0 {
						t.Fatal(plan)
					}
				} else if plan.IntegerLowerBound != nil || plan.IntegerUpperExclusive != nil {
					t.Fatal("unresolved target acquired bounds")
				}
			case NumericConvertSignedToFloat, NumericConvertUnsignedToFloat:
				if !reflect.DeepEqual(plan.Checks, []string{"finite-range", "significand"}) {
					t.Fatal(plan.Checks)
				}
			case NumericConvertFloatToBool:
				if plan.RuntimeCheck || plan.BoolPredicate != "une-zero" {
					t.Fatal(plan)
				}
			default:
				if !reflect.DeepEqual(plan.Checks, []string{"map-nan-infinity", "finite-range", "round-trip"}) {
					t.Fatal(plan.Checks)
				}
			}
			testNumericPlanDetached(t, a, call, plan)
		}
		if !wideExact || len(categories) != 3 {
			t.Fatalf("wide exact=%v, categories=%v", wideExact, categories)
		}
	}
}

// try cannot hide a conversion statically known to fail. Canonical failure
// reasons retain priority; no successful plan is published for these calls.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§41,47–59;
// rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md — §2.
func TestPrimitiveNumericConversionConstantFailures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_validation_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		want := []string{"PrecisionLoss", "OutOfRange", "PrecisionLoss", "OutOfRange", "PrecisionLoss", "OutOfRange", "OutOfRange", "PrecisionLoss", "OutOfRange", "NonFinite", "NonFinite", "NonFinite", "NonFinite", "PrecisionLoss"}
		var found []string
		for _, err := range errors {
			if !strings.Contains(err.Message, "constant conversion from") {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
			found = append(found, strings.Split(err.Message, "ConversionError.")[1])
		}
		if !reflect.DeepEqual(found, want) {
			t.Fatalf("width %d: reasons=%v expected=%v", width, found, want)
		}
		for call := range a.resolvedNumericConversionPlans {
			plan, _ := a.ResolvedNumericConversionPlanOf(call)
			if plan.TargetType.Kind != FloatType {
				t.Fatal("invalid conversions published plans")
			}
		}
	}
}
