package sema

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const checkedNumericConversionSource = `
module main

fn FloatNarrow(value: float64) float32 {
	return float32(value)
}

fn FloatWiden(value: float32) float64 {
	return float64(value)
}

fn FloatToInteger(value: float64) int64 {
	return int64(value)
}

fn SmallIntegerToFloat(value: int16, wide: uint32) float64 {
	let narrow: float32 := float32(value)
	return float64(wide) + float64(narrow)
}

fn WideIntegerToFloat(value: int64) float64 {
	return float64(value)
}

fn MediumIntegerToFloat32(value: int32) float32 {
	return float32(value)
}

fn HugeIntegerToFloat32(value: uint256) float32 {
	return float32(value)
}

fn ProvenIntegerToFloat(value: int64) float64 {
	if value >= 0 && value <= 1000 {
		return float64(value)
	}
	return 0.0
}

fn FloatToDecimal(value: float64) decimal {
	return decimal(value)
}

fn DecimalToFloat(value: decimal) float64 {
	return float64(value)
}

fn DecimalNarrow(value: decimal128) decimal {
	return decimal(value)
}

fn DecimalWiden(value: decimal) decimal128 {
	return decimal128(value)
}

fn IntegerToDecimal(value: int64, wide: uint64) decimal {
	let exact: decimal := decimal(value)
	let exactWide: decimal128 := decimal128(wide)
	return decimal(wide)
}

fn Protected(value: float64, amount: decimal) Result[int32, ConversionError] {
	let narrow: float32 := try float32(value)
	let whole: int32 := try int32(amount)
	let handled: int32 := try int32(value) {
		Err(_) => 0
	}
	return Ok(whole + handled)
}
`

// Every Sec-only conversion involving a binary floating or decimal type is
// checked: it requires try exactly when the target cannot represent every
// value the source may hold.
//
// Rules:
//   - rules/types/types.md — § 25(3)–(5), § 25(7) "Explicit conversions"
//   - rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§ 12–15
func TestCheckedNumericConversionsRequireTry(t *testing.T) {
	errors := analyzeSourceRaw(t, checkedNumericConversionSource)
	assertSemaErrors(t, errors, []string{
		"converting float64 to float32 may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 5:9",
		"converting float64 to int64 may fail with ConversionError.NonFinite, ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 13:9",
		"converting int64 to float64 may fail with ConversionError.PrecisionLoss and requires try at 22:9",
		"converting int32 to float32 may fail with ConversionError.PrecisionLoss and requires try at 26:9",
		"converting uint256 to float32 may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 30:9",
		"converting float64 to decimal may fail with ConversionError.NonFinite or ConversionError.PrecisionLoss and requires try at 41:9",
		"converting decimal to float64 may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 45:9",
		"converting decimal128 to decimal may fail with ConversionError.PrecisionLoss and requires try at 49:9",
		"converting uint64 to decimal may fail with ConversionError.PrecisionLoss and requires try at 59:9",
	})
}

// Plain float is target-sized (MD-014): the selected plan decides, and
// without a plan a conversion is exact only when it is at both widths.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 6.3–6.9
//   - rules/types/types.md — § 25.1(2)
func TestCheckedPlatformFloatConversionsFollowThePlan(t *testing.T) {
	const source = `
module main

fn ToPlatform(value: float64) float {
	return float(value)
}

fn FromPlatform(value: float) float32 {
	return float32(value)
}

fn Widen(value: float32, platform: float) float64 {
	let same: float := float(value)
	return float64(platform) + float64(same)
}
`
	toPlatform := "converting float64 to float may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 5:9"
	fromPlatform := "converting float to float32 may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 9:9"
	tests := []struct {
		name     string
		analyzer *Analyzer
		want     []string
	}{
		{"no plan", NewAnalyzer(), []string{toPlatform, fromPlatform}},
		{"32-bit", NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 32}), []string{toPlatform}},
		{"64-bit", NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64}), []string{fromPlatform}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parser.New(lexer.New(source))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			assertSemaErrors(t, test.analyzer.Analyze(program), test.want)
		})
	}
}

// A compile-time-known value is proved by its exact value, so an exactly
// representable constant needs no try while an inexact one still does.
//
// Rules:
//   - rules/types/types.md — § 25(7) "Explicit conversions"
func TestCheckedNumericConversionConstantProof(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Test() float32 {
	let half: float64 := 0.5
	let tenth: float64 := 0.1
	let whole: float64 := 42.0
	let amount: decimal := 12.25
	let count: int32 := int32(whole)
	let exact: decimal := decimal(half)
	let binary: float64 := float64(amount)
	let narrow: float32 := float32(half)
	let fraction: int32 := int32(half)
	return float32(tenth)
}
`)
	assertSemaErrors(t, errors, []string{
		"converting float64 to int32 may fail with ConversionError.NonFinite, ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 13:25",
		"converting float64 to float32 may fail with ConversionError.OutOfRange or ConversionError.PrecisionLoss and requires try at 14:9",
	})
}

// The recorded fact lists every variant in runtime priority order and is
// the same list lowering and tooling read.
//
// Rules:
//   - rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — § 12 "Error priority"
func TestCheckedNumericConversionFailureFacts(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Test(value: float64) Result[int32, ConversionError] {
	return Ok(try int32(value))
}
`)
	assertSemaErrors(t, errors, nil)
	var found bool
	for expression := range analyzer.checkedConversions {
		call, ok := expression.(*ast.CallExpression)
		if !ok || !strings.HasPrefix(call.String(), "int32(") {
			continue
		}
		found = true
		want := []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}
		if got := analyzer.CheckedConversionFailuresOf(call); !reflect.DeepEqual(got, want) {
			t.Fatalf("failures = %v, want %v", got, want)
		}
		if primary, ok := analyzer.CheckedConversionOf(call); !ok || primary != ConversionFailureNonFinite {
			t.Fatalf("primary = %q, %v", primary, ok)
		}
	}
	if !found {
		t.Fatal("checked int32(value) conversion was not recorded")
	}
}

// The pair table itself, independent of source analysis.
func TestNumericConversionFailureTable(t *testing.T) {
	float32Type := Type{Name: "float32", Kind: FloatType, FloatBits: 32}
	float64Type := Type{Name: "float64", Kind: FloatType, FloatBits: 64}
	decimal64 := Type{Name: "decimal", Kind: DecimalType}
	decimal128 := Type{Name: "decimal128", Kind: DecimalType}
	integer := signedType("int64", -1<<63, 1<<63-1)
	precision := new(big.Int).Lsh(big.NewInt(1), 53)
	beyond := new(big.Int).Add(precision, big.NewInt(1))
	tests := []struct {
		name             string
		target, source   Type
		minimum, maximum *big.Int
		want             []ConversionFailureKind
	}{
		{"float widening", float64Type, float32Type, nil, nil, nil},
		{"float narrowing", float32Type, float64Type, nil, nil, []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}},
		{"integer at float precision", float64Type, integer, new(big.Int).Neg(precision), precision, nil},
		{"integer beyond float precision", float64Type, integer, big.NewInt(0), beyond, []ConversionFailureKind{ConversionFailurePrecisionLoss}},
		{"decimal widening", decimal128, decimal64, nil, nil, nil},
		{"decimal narrowing", decimal64, decimal128, nil, nil, []ConversionFailureKind{ConversionFailurePrecisionLoss}},
		{"integer in coefficient", decimal64, integer, integer.MinInteger, integer.MaxInteger, nil},
		{"float to decimal", decimal128, float32Type, nil, nil, []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailurePrecisionLoss}},
	}
	for _, test := range tests {
		got, handled := numericConversionFailures(test.target, test.source, test.minimum, test.maximum)
		if !handled || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %v handled=%v, want %v", test.name, got, handled, test.want)
		}
	}
	if _, handled := numericConversionFailures(integer, integer, nil, nil); handled {
		t.Error("an integer pair belongs to the integer-family rules")
	}
}
