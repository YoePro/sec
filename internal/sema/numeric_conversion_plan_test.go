package sema

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestResolvedNumericConversionPlans covers the complete P20 primitive pair
// matrix at both selected widths and without a plan, at every analysis depth.
// Expected failure categories come from format precision/range, not the producer.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–61;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§12–15.
func TestResolvedNumericConversionPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("width%d/depth%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.New(string(data)))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				seen := map[string]bool{}
				var first ast.Expression
				for expression := range a.expressionTypes {
					plan, ok := a.ResolvedNumericConversionPlanOf(expression)
					if !ok {
						continue
					}
					first = expression
					call := expression.(*ast.CallExpression)
					if call.Arguments[0].String() != "value" {
						if plan.Proof.Basis != "constant" || plan.Proof.Constant.Cmp(big.NewRat(1, 2)) != 0 {
							t.Fatalf("constant proof: %+v", plan)
						}
						testNumericPlanDetached(t, a, expression, plan)
						continue
					}
					key := plan.SourceType.Name + "/" + plan.TargetType.Name
					// The bounded int256 call has a path proof, independent of
					// the full-domain matrix call of the same source/target pair.
					if plan.Proof.Basis == "value-interval" && plan.SourceType.Name == "int256" {
						if plan.Proof.SourceMin.Sign() != 0 || plan.Proof.SourceMax.Cmp(big.NewInt(100)) != 0 {
							t.Fatal(plan.Proof)
						}
						testNumericPlanDetached(t, a, expression, plan)
						continue
					}
					seen[key] = true
					sb, tb := numericPlanExpectedBits(plan.SourceType.Name, width), numericPlanExpectedBits(plan.TargetType.Name, width)
					if plan.SourceBits != sb || plan.TargetBits != tb || plan.PlanWidthBits != width {
						t.Fatalf("%s: widths %+v, expected %d/%d", key, plan, sb, tb)
					}
					if plan.SourceType.Name == "float" && plan.SourceType.FloatBits != sb || plan.TargetType.Name == "float" && plan.TargetType.FloatBits != tb {
						t.Fatalf("platform snapshot %s: bits %d/%d Type bits %d/%d", key, sb, tb, plan.SourceType.FloatBits, plan.TargetType.FloatBits)
					}
					kind, failures := numericPlanExpected(plan.SourceType.Name, plan.TargetType.Name, sb, tb)
					if plan.Kind != kind || !reflect.DeepEqual(plan.Failures, failures) || plan.RuntimeCheck != (len(failures) != 0) || plan.ProvenExact != (len(failures) == 0) {
						t.Fatalf("%s: %+v, expected %s failures %v", key, plan, kind, failures)
					}
					if plan.RuntimeCheck && (plan.ErrorType.Name != "ConversionError" || plan.Proof.Basis != "") {
						t.Fatalf("checked error/proof %s: %+v", key, plan)
					}
					if plan.ProvenExact && (plan.ErrorType.Name != "" || plan.Proof.Basis == "") {
						t.Fatalf("exact error/proof %s: %+v", key, plan)
					}
					testNumericPlanDetached(t, a, expression, plan)
				}
				// 9 float pairs + 14 integer types * 3 floats * 2
				// directions + 3 float-to-bool pairs.
				if len(seen) != 96 || len(a.resolvedNumericConversionPlans) != 98 {
					t.Fatalf("matrix coverage: %d pairs, %d calls", len(seen), len(a.resolvedNumericConversionPlans))
				}
				before := len(a.resolvedNumericConversionPlans)
				for _, expression := range []ast.Expression{nil, &ast.CallExpression{}, &ast.Identifier{}} {
					if _, ok := a.ResolvedNumericConversionPlanOf(expression); ok {
						t.Fatal("unresolved expression acquired a plan")
					}
				}
				if len(a.resolvedNumericConversionPlans) != before {
					t.Fatal("query changed published plans")
				}
				a.Analyze(&ast.Program{})
				if _, ok := a.ResolvedNumericConversionPlanOf(first); ok || len(a.resolvedNumericConversionPlans) != 0 {
					t.Fatal("old plans survived reanalysis")
				}
			})
		}
	}
	var absent *Analyzer
	if _, ok := absent.ResolvedNumericConversionPlanOf(&ast.CallExpression{}); ok {
		t.Fatal("nil analyzer has a plan")
	}
}

func numericPlanExpectedBits(name string, width uint16) int {
	switch name {
	case "float", "int", "uint":
		return int(width)
	case "bool":
		return 1
	}
	var bits int
	fmt.Sscanf(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(name, "uint"), "int"), "float"), "%d", &bits)
	return bits
}

func numericPlanExpected(source, target string, sb, tb int) (NumericConversionKind, []ConversionFailureKind) {
	sf, tf := strings.HasPrefix(source, "float"), strings.HasPrefix(target, "float")
	if sf && tf {
		kind := NumericConvertFloatToFloat
		if sb != 0 && tb != 0 {
			if sb < tb {
				kind = NumericConvertFloatWiden
			} else if sb > tb {
				kind = NumericConvertFloatNarrow
			}
		}
		if source != target && (sb == 0 && tb == 32 || tb == 0 && sb == 64 || sb > tb && tb != 0) {
			return kind, []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}
		}
		return kind, nil
	}
	if target == "bool" {
		return NumericConvertFloatToBool, nil
	}
	if sf {
		kind := NumericConvertFloatToSigned
		if strings.HasPrefix(target, "uint") {
			kind = NumericConvertFloatToUnsigned
		}
		return kind, []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}
	}
	kind := NumericConvertSignedToFloat
	if strings.HasPrefix(source, "uint") {
		kind = NumericConvertUnsignedToFloat
	}
	if sb == 0 {
		sb = 64 // Portable proof must hold for both int/uint widths.
	}
	precision, exponent := 24, 128
	if tb == 64 {
		precision, exponent = 53, 1024
	}
	if sb <= precision {
		return kind, nil
	}
	if sb > exponent || sb == exponent && strings.HasPrefix(source, "uint") {
		return kind, []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}
	}
	return kind, []ConversionFailureKind{ConversionFailurePrecisionLoss}
}

func testNumericPlanDetached(t *testing.T, a *Analyzer, expression ast.Expression, want ResolvedNumericConversionPlan) {
	t.Helper()
	plan, _ := a.ResolvedNumericConversionPlanOf(expression)
	plan.SourceType.Name, plan.TargetType.Name, plan.ErrorType.Name = "corrupt", "corrupt", "corrupt"
	if plan.SourceType.MinInteger != nil {
		plan.SourceType.MinInteger.SetInt64(777)
	}
	if plan.TargetType.MaxInteger != nil {
		plan.TargetType.MaxInteger.SetInt64(777)
	}
	if len(plan.ErrorType.UnionVariants) > 0 {
		plan.ErrorType.UnionVariants[0].Name = "corrupt"
	}
	if len(plan.Checks) > 0 {
		plan.Checks[0] = "corrupt"
	}
	if plan.IntegerLowerBound != nil {
		plan.IntegerLowerBound.SetInt64(777)
		plan.IntegerUpperExclusive.SetInt64(777)
	}
	if len(plan.Failures) > 0 {
		plan.Failures[0] = "corrupt"
	}
	if plan.Proof.Constant != nil {
		plan.Proof.Constant.SetInt64(777)
	}
	if plan.Proof.SourceMin != nil {
		plan.Proof.SourceMin.SetInt64(777)
		plan.Proof.SourceMax.SetInt64(777)
	}
	again, _ := a.ResolvedNumericConversionPlanOf(expression)
	if !reflect.DeepEqual(again, want) {
		t.Fatal("query aliases published plan")
	}
}

// Invalid casts cannot acquire a valid P20 plan merely by being queried.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§42–43,61.
func TestResolvedNumericConversionPlansInvalid(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/numeric_conversion_plans_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("invalid conversion accepted")
	}
	if len(a.resolvedNumericConversionPlans) != 0 {
		t.Fatal("invalid conversion published a plan")
	}
	for expression := range a.expressionTypes {
		if _, ok := a.ResolvedNumericConversionPlanOf(expression); ok {
			t.Fatal("query created a plan for invalid conversion")
		}
	}
}
