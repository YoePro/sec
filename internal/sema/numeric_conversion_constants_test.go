package sema

import (
	"math"
	"math/big"
	"os"
	"sec/internal/ast"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestNumericConversionKnownInvalidValues requires intrinsic constant failures
// to remain compile-time errors under try, with range before precision and
// without a successful checked/numeric conversion plan for the rejected call.
// Rules: rules/types/types.md — §25(9–11);
// rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md — §4.8;
// rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md — §2.
func TestNumericConversionKnownInvalidValues(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_constants_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PrecisionLoss", "OutOfRange", "OutOfRange", "PrecisionLoss", "OutOfRange", "PrecisionLoss", "PrecisionLoss", "NonFinite", "PrecisionLoss", "PrecisionLoss", "PrecisionLoss", "PrecisionLoss"}
	for _, width := range []uint16{0, 32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "numeric_conversion_constants_invalid.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		if len(errors) != len(want) {
			t.Fatalf("width %d: got %d failures, want %d: %+v", width, len(errors), len(want), errors)
		}
		if len(a.resolvedNumericConversionPlans) != 0 || len(a.checkedConversions) != 0 || len(a.fallibleConversionRelations) != 0 {
			t.Fatal("rejected conversions published successful runtime plans")
		}
		for i, e := range errors {
			if !strings.Contains(e.Message, "constant conversion from") || !strings.HasSuffix(e.Message, "ConversionError."+want[i]) {
				t.Fatalf("width %d, diagnostic %d: %+v", width, i, e)
			}
		}
	}
}

// TestNumericConversionKnownValidAndRuntimeValues preserves exact conversions
// and runtime checks where mutable or parameter values have no constant proof.
// Rules: rules/types/types.md — §25(5,7,10–11);
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§2–3.
func TestNumericConversionKnownValidAndRuntimeValues(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_constants_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "numeric_conversion_constants_valid.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatalf("width %d: %+v", width, errors)
		}
		runtime := 0
		for _, conversion := range a.checkedConversions {
			if len(conversion.failures) > 0 {
				runtime++
			}
		}
		if runtime != 3 {
			t.Fatalf("width %d: got %d runtime checks, want 3", width, runtime)
		}
	}
}

// TestNumericConversionConstantTargetWidths proves failure only if every
// unresolved target fails; it retains each possible cause when they differ.
// Rules: rules/types/types.md — §25(10–11), §25.1(2);
// rules/platform/target_profiles.md — CompilationPlan-owned widths.
func TestNumericConversionConstantTargetWidths(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_platform_constant.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "numeric_conversion_platform_constant.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		want := []string{"OutOfRange or ConversionError.PrecisionLoss"}
		if width == 32 {
			want = []string{"OutOfRange", "OutOfRange"}
		}
		if width == 64 {
			want = []string{"PrecisionLoss"}
		}
		if len(errors) != len(want) {
			t.Fatalf("width %d: %+v", width, errors)
		}
		for i, e := range errors {
			if !strings.HasSuffix(e.Message, "ConversionError."+want[i]) {
				t.Fatalf("width %d: %+v", width, e)
			}
		}
	}
}

// TestDecimalConstantConversionExtremeScales exercises the full int32 scale
// domain without constructing powers of ten proportional to the source scale.
// Decimal narrowing must preserve the asymmetric coefficient boundary and may
// strip exact trailing zeros only while the destination scale stays valid.
// Rules: rules/types/types.md — §25(10–11);
// rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§2,3(3–7).
func TestDecimalConstantConversionExtremeScales(t *testing.T) {
	for _, tc := range []struct {
		coefficient string
		scale       int32
		target      string
		want        ConversionFailureKind
	}{
		{"1", math.MinInt32, "int256", ConversionFailureOutOfRange},
		{"1", math.MinInt32, "float64", ConversionFailureOutOfRange},
		{"1", math.MaxInt32, "int32", ConversionFailurePrecisionLoss},
		{"-1", math.MaxInt32, "uint256", ConversionFailureOutOfRange},
		{"1", math.MaxInt32, "float64", ConversionFailurePrecisionLoss},
		{"0", math.MaxInt32, "int32", ""},
		{"0", math.MinInt32, "float32", ""},
		{"92233720368547758070", math.MinInt32, "decimal", ConversionFailurePrecisionLoss},
		{"92233720368547758070", math.MinInt32 + 1, "decimal", ""},
		{"-9223372036854775808", math.MaxInt32, "decimal", ""},
	} {
		a := NewAnalyzer()
		a.symbols = newSymbolTable()
		coefficient, ok := new(big.Int).SetString(tc.coefficient, 10)
		if !ok {
			t.Fatal(tc.coefficient)
		}
		a.symbols.set("value", Symbol{Name: "value", Type: a.types["decimal128"], DecimalExact: &DecimalValue{Coefficient: coefficient, Scale: tc.scale}})
		got, known := a.decimalConstantConversionFailure(a.types[tc.target], &ast.Identifier{Value: "value"})
		if !known || got != tc.want {
			t.Fatalf("%s scale %d -> %s: %s, known %v; want %s", tc.coefficient, tc.scale, tc.target, got, known, tc.want)
		}
	}
}
