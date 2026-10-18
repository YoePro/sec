package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
)

// Float operators resolve to their P20 kinds, an untyped literal beside a
// float operand or in a float context is shaped to that float type, float
// compound assignment shares the operator, and decimal remainder and an
// unrelated decimal mix are typed or diagnosed instead of silently invalid.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 11, 20–26
//   - rules/types/types.md — "Context shaping"
//   - rules/foundations/operators.md — "Decimal remainder"
func TestFloatOperatorsAndLiteralShaping(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main
fn F(a: float64, b: float32) bool {
    let mut total: float64 := 3.25
    total += 1.5
    let negative := -a
    return a + 2.0 > total && b % 2.0 != 0.5 && -1.0e10 < a
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	kinds := map[ResolvedOperatorKind]bool{}
	for _, resolved := range analyzer.resolvedOperators {
		kinds[resolved.Kind] = true
	}
	for _, want := range []ResolvedOperatorKind{ResolvedFloatAdd, ResolvedFloatRemainder, ResolvedFloatNegate, ResolvedFloatCompareGT, ResolvedFloatCompareNE, ResolvedFloatCompareLT} {
		if !kinds[want] {
			t.Errorf("missing %s in %v", want, kinds)
		}
	}
	for expr, typ := range analyzer.expressionTypes {
		if literal, ok := expr.(*ast.FloatLiteral); ok && typ.Kind != FloatType {
			t.Errorf("literal %s typed %s, want a float type", literal.String(), typ.Name)
		}
	}
	compound := false
	for _, resolved := range analyzer.resolvedCompoundAssignments {
		compound = compound || resolved.Kind == ResolvedFloatAdd
	}
	if !compound {
		t.Error("float compound assignment not resolved")
	}

	if errors := analyzeSourceRaw(t, "module main\nfn R() bool {\n    return -7.5 % 2.0 != -1.5\n}\n"); len(errors) != 0 {
		t.Fatalf("decimal remainder rejected: %v", errors)
	}
	errors = analyzeSourceRaw(t, "module main\nfn M(a: float64, d: decimal) float64 {\n    return a + d\n}\n")
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "cannot apply operator + to float64 and decimal") {
		t.Fatalf("float/decimal mix not diagnosed: %v", errors)
	}
}
