package sema

import (
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

// TestShapedMultiplyFacts verifies canonical shape/scalar decisions, corrected
// result counts, detached query data, all depths and clean analyzer reuse.
// Rules: rules/collections/shaped-types.md — §§4–5,22,29,35.
func TestShapedMultiplyFacts(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		program, expressions := shapedFactFixture(t, "shaped_multiply_facts_valid.sec")
		a := NewAnalyzerWithDepth(depth)
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		counts := []int64{8, 4, 0, 12}
		plans := []ResolvedShapedMultiply{}
		for index, expression := range expressions {
			before := len(a.errors)
			plan, found := a.ResolvedShapedMultiplyOf(expression)
			if !found || len(a.errors) != before || plan.ShapeCheck != "proven-safe" || plan.ResultType.StaticElementCount.Int64() != counts[index] || plan.ProductElement.Kind != plan.AccumulatorElement.Kind {
				t.Fatal(index, plan, a.errors)
			}
			if plan.Rows != plan.LeftType.ConstArgs[0] || plan.Inner != plan.LeftType.ConstArgs[1] || plan.Source != expression.Token {
				t.Fatal(plan)
			}
			if index == 1 && (plan.Kind != "matrix-vector" || plan.Columns != 1) {
				t.Fatal(plan)
			}
			if index != 1 && plan.Kind != "matrix-matrix" {
				t.Fatal(plan)
			}
			if index == 0 && (plan.ProductElement.FloatBits != 32 || plan.ResultType.TypeArgs[0].FloatBits != 32) {
				t.Fatal(plan)
			}
			resolved, _ := a.ResolvedTypeOf(expression)
			if resolved.StaticElementCount.Cmp(plan.ResultType.StaticElementCount) != 0 {
				t.Fatal("expression type disagrees with plan", resolved, plan)
			}
			plans = append(plans, plan)
			mutated, _ := a.ResolvedShapedMultiplyOf(expression)
			mutated.ResultType.ConstArgs[0] = 99
			mutated.ResultType.StaticElementCount.SetInt64(99)
			mutated.LeftType.ConstArgs[0] = 99
			mutated.RightType.TypeArgs[0].Name = "corrupted"
			if mutated.ProductElement.MaxInteger != nil {
				mutated.ProductElement.MaxInteger.SetInt64(0)
			}
			fresh, _ := a.ResolvedShapedMultiplyOf(expression)
			if !reflect.DeepEqual(plan, fresh) {
				t.Fatal("query mutated canonical facts", plan, fresh)
			}
		}
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		for index, expression := range expressions {
			plan, _ := a.ResolvedShapedMultiplyOf(expression)
			if !reflect.DeepEqual(plans[index], plan) {
				t.Fatal("reanalysis changed plan")
			}
		}
		empty := parser.New(lexer.New("module main\n")).ParseProgram()
		a.Analyze(empty)
		if _, found := a.ResolvedShapedMultiplyOf(expressions[0]); found {
			t.Fatal("stale plan")
		}
		if _, found := a.ResolvedShapedMultiplyOf(nil); found {
			t.Fatal("nil expression acquired plan")
		}
	}
}

// TestShapedMultiplyInvalidFacts rejects invalid algebra and target-overflowing
// derived results without publishing successful plans for them.
// Rules: rules/collections/shaped-types.md — §§4–5,22,29;
// rules/corrections/applied/correction27-20260823.md — Bugs 1–2.
func TestShapedMultiplyInvalidFacts(t *testing.T) {
	program, expressions := shapedFactFixture(t, "shaped_multiply_facts_invalid.sec")
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 3 {
		t.Fatal(errs)
	}
	for _, expression := range expressions {
		if _, found := a.ResolvedShapedMultiplyOf(expression); found {
			t.Fatal("invalid plan published")
		}
	}
	for _, bits := range []int{32, 64} {
		program, expressions := shapedFactFixture(t, "shaped_multiply_target_extent.sec")
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: uint16(bits)})
		errs := a.Analyze(program)
		plan, found := a.ResolvedShapedMultiplyOf(expressions[0])
		if bits == 32 {
			if len(errs) != 1 || !strings.Contains(errs[0].Message, "4294967296 overflows target uint") || found {
				t.Fatal(errs, plan)
			}
		} else if len(errs) != 0 || !found || plan.ResultType.StaticElementCount.Cmp(new(big.Int).Lsh(big.NewInt(1), 32)) != 0 {
			t.Fatal(errs, plan)
		}
	}
}

// shapedFactFixture reads source fixtures and collects the exact operator nodes
// used as keys by the canonical Sema query.
// Rules: rules/collections/shaped-types.md — §22.
func shapedFactFixture(t *testing.T, name string) (*ast.Program, []*ast.InfixExpression) {
	t.Helper()
	file := "../../testdata/sema/" + name
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	var expressions []*ast.InfixExpression
	walkASTValue(reflect.ValueOf(program), func(value any) {
		if expr, ok := value.(*ast.InfixExpression); ok && expr.Operator == "x" {
			expressions = append(expressions, expr)
		}
	})
	return program, expressions
}
