package sema

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Package 14 sections 9-17: fixed length is exact, shape is explicit, and the
// same source is validated independently against each target uint plan.
func TestCanonicalArrayShapeAndTargetLengthMatrix(t *testing.T) {
	const source = `module main

fn Zero(value: int[0]) void {}
fn Expr(value: int[2 + 3]) void {}
fn Wide(value: int[9223372036854775808]) void {}
fn Max64(value: int[18446744073709551615]) void {}
`

	analyzer64, errors64 := analyzeArraySourceWithWidth(t, source, 64)
	if len(errors64) != 0 {
		t.Fatalf("64-bit array lengths produced errors: %v", errors64)
	}
	wide := analyzer64.Functions()["Wide"][0].Parameters[0].Type
	expressionLength, _ := FixedArrayLength(analyzer64.Functions()["Expr"][0].Parameters[0].Type)
	if expressionLength.String() != "5" {
		t.Fatalf("ordinary exact expression length = %s, want 5", expressionLength)
	}
	if wide.ArrayShape != ArrayShapeFixed || wide.ArrayLengthDecimal != "9223372036854775808" {
		t.Fatalf("wide fixed-array fact = %#v", wide)
	}
	if wide.ArrayLength == dynamicArrayLength {
		t.Fatal("wide fixed array leaked the legacy dynamic sentinel")
	}
	length, ok := FixedArrayLength(wide)
	if !ok || length.String() != "9223372036854775808" {
		t.Fatalf("exact wide length = %v, %v", length, ok)
	}
	length.SetInt64(1)
	again, _ := FixedArrayLength(wide)
	if again.String() != "9223372036854775808" {
		t.Fatal("FixedArrayLength did not return a defensive copy")
	}
	if err := ValidateArrayTypeForScalarPlan(wide, layout.ResolvedScalarPlan{PointerWidthBits: 64}); err != nil {
		t.Fatalf("same fact rejected by uint64 plan: %v", err)
	}
	if err := ValidateArrayTypeForScalarPlan(wide, layout.ResolvedScalarPlan{PointerWidthBits: 32}); err == nil {
		t.Fatal("same fact accepted by uint32 plan")
	}

	_, errors32 := analyzeArraySourceWithWidth(t, source, 32)
	if got := joinedSemaErrors(errors32); !strings.Contains(got, "fixed-array length 9223372036854775808 overflows target uint32") ||
		!strings.Contains(got, "fixed-array length 18446744073709551615 overflows target uint32") {
		t.Fatalf("32-bit plan diagnostics = %v", errors32)
	}
}

func TestCanonicalArrayLengthRejectsOneAboveUint64WithoutHostConversion(t *testing.T) {
	_, errors := analyzeArraySourceWithWidth(t, `module main
fn TooWide(value: int[18446744073709551616]) void {}
`, 64)
	if got := joinedSemaErrors(errors); !strings.Contains(got, "fixed-array length 18446744073709551616 overflows target uint64") {
		t.Fatalf("uint64 overflow diagnostics = %v", errors)
	}
}

func TestCanonicalArrayLengthUint32BoundaryAndExpressionErrors(t *testing.T) {
	const boundary = `module main
fn Max32(value: int[4294967295]) void {}
fn Above32(value: int[4294967296]) void {}
`
	_, errors32 := analyzeArraySourceWithWidth(t, boundary, 32)
	if got := joinedSemaErrors(errors32); strings.Contains(got, "4294967295 overflows") ||
		!strings.Contains(got, "fixed-array length 4294967296 overflows target uint32") {
		t.Fatalf("uint32 boundary diagnostics = %v", errors32)
	}
	_, errors64 := analyzeArraySourceWithWidth(t, boundary, 64)
	if len(errors64) != 0 {
		t.Fatalf("uint64 plan rejected uint32 boundary cases: %v", errors64)
	}

	_, invalid := analyzeArraySourceWithWidth(t, `module main
fn Negative(value: int[-1]) void {}
fn Runtime(n: int, value: int[1 + n]) void {}
fn NonInteger(value: int[1.5]) void {}
`, 64)
	got := joinedSemaErrors(invalid)
	for _, want := range []string{"array length must be non-negative", "array length must be a compile-time integer"} {
		if !strings.Contains(got, want) {
			t.Fatalf("invalid length diagnostics %q missing %q", got, want)
		}
	}
	if strings.Count(got, "array length must be a compile-time integer") < 2 {
		t.Fatalf("runtime and non-integer lengths were not diagnosed independently: %q", got)
	}
}

// TestPackage14ArrayDiagnosticsAreDeterministicAcrossPlans rebuilds the same
// invalid source for alternating target widths. Diagnostic source order and
// exact decimal values must depend only on source plus the selected plan, never
// on host word size or internal map traversal.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 10, 60, 105
func TestPackage14ArrayDiagnosticsAreDeterministicAcrossPlans(t *testing.T) {
	const source = `module main
fn Above32(value: int[4294967296]) void {}
fn Above64(value: int[18446744073709551616]) void {}
fn Negative(value: int[-1]) void {}
fn Runtime(n: int, value: int[1 + n]) void {}
`
	baselines := map[uint16]string{}
	for iteration := 0; iteration < 12; iteration++ {
		widths := []uint16{32, 64}
		if iteration%2 != 0 {
			widths[0], widths[1] = widths[1], widths[0]
		}
		for _, width := range widths {
			_, diagnostics := analyzeArraySourceWithWidth(t, source, width)
			got := joinedSemaErrors(diagnostics)
			if baseline, exists := baselines[width]; exists {
				if got != baseline {
					t.Fatalf("iteration %d uint%d diagnostics changed:\n%s\n--- baseline ---\n%s", iteration, width, got, baseline)
				}
			} else {
				baselines[width] = got
			}
		}
	}
	want32 := []string{
		"fixed-array length 4294967296 overflows target uint32",
		"fixed-array length 18446744073709551616 overflows target uint32",
		"array length must be non-negative",
		"array length must be a compile-time integer",
	}
	want64 := []string{
		"fixed-array length 18446744073709551616 overflows target uint64",
		"array length must be non-negative",
		"array length must be a compile-time integer",
	}
	for width, expected := range map[uint16][]string{32: want32, 64: want64} {
		position := -1
		for _, message := range expected {
			next := strings.Index(baselines[width][position+1:], message)
			if next < 0 {
				t.Fatalf("uint%d diagnostics missing %q in source order:\n%s", width, message, baselines[width])
			}
			position += next + 1
		}
	}
}

func TestCanonicalArrayIdentityUsesExactDecimalLength(t *testing.T) {
	element := Type{Name: "int", Kind: IntType}
	a := NewFixedArrayType(element, mustArrayLength(t, "9223372036854775808"))
	b := NewFixedArrayType(element, mustArrayLength(t, "9223372036854775809"))
	if sameConcreteType(a, b) || canonicalTypeIdentity(a) == canonicalTypeIdentity(b) {
		t.Fatal("distinct above-int64 array lengths collapsed to one type identity")
	}
	dynamic := NewDynamicArrayType(element)
	if sameConcreteType(a, dynamic) || dynamic.ArrayShape != ArrayShapeDynamic || dynamic.ArrayLengthDecimal != "" {
		t.Fatal("fixed and dynamic array shapes were conflated")
	}
}

func TestHugeFixedArrayDefaultRemainsCompact(t *testing.T) {
	element := Type{Name: "int", Kind: IntType}
	typ := NewFixedArrayType(element, mustArrayLength(t, "9223372036854775808"))
	resolved := DefaultValueOf(typ)
	if resolved.Kind != ArrayDefault || resolved.ArrayLengthDecimal != "9223372036854775808" ||
		resolved.ArrayElementDefault == nil || len(resolved.Elements) != 0 {
		t.Fatalf("huge compact default = %#v", resolved)
	}
}

func analyzeArraySourceWithWidth(t *testing.T, source string, width uint16) (*Analyzer, []Error) {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	return analyzer, analyzer.Analyze(program)
}

func joinedSemaErrors(errors []Error) string {
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		parts = append(parts, err.Message)
	}
	return strings.Join(parts, "\n")
}

func mustArrayLength(t *testing.T, value string) *big.Int {
	t.Helper()
	length, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid test length %q", value)
	}
	return length
}
