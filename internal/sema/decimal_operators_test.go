package sema

import "testing"

// MD-066/MD-067 decimal operator plans: checked arithmetic kinds with
// preferred-scale rules and possible failures, folded constant results with
// their selected scale, scale-independent comparison, and try over decimal
// arithmetic.
//
// Rules:
//   - rules/foundations/operators.md — "Decimal arithmetic", "Decimal result scale", "Decimal division"
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 3–7
func TestDecimalOperatorPlans(t *testing.T) {
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Plans(a: decimal, b: decimal, wide: decimal128) bool {
    let sum := a + b
    let product := a * b
    let quotient := a / b
    let remainder := a % b
    let negated := -a
    let widened := wide * 2
    return sum == product
}

fn Constants() decimal {
    let third := 1m / 3m
    let exact := 1m / 8m
    let tooPrecise := third * 3m
    let product := 1.5m * 2.50m
    return exact
}

fn Checked(a: decimal, b: decimal) Result[decimal, ArithmeticError] {
    let value := try a / b
    return Ok(value)
}
`)
	assertSemaErrors(t, errs, nil)
	plans := map[string]ResolvedOperator{}
	for expr, resolved := range analyzer.resolvedOperators {
		if resolved.Decimal != nil {
			plans[expr.String()] = resolved
		}
	}
	expect := func(source string, kind ResolvedOperatorKind) ResolvedOperator {
		t.Helper()
		plan, ok := plans[source]
		if !ok {
			t.Fatalf("no decimal plan for %s in %v", source, plans)
		}
		if plan.Kind != kind {
			t.Fatalf("%s kind = %s, want %s", source, plan.Kind, kind)
		}
		return plan
	}
	if plan := expect("(a + b)", ResolvedDecimalAddChecked); !plan.RuntimeCheck || plan.Decimal.PreferredScale != DecimalPreferredMaxScale || plan.Decimal.CoefficientBits != 64 {
		t.Fatalf("add plan = %+v %+v", plan, plan.Decimal)
	}
	if plan := expect("(a * b)", ResolvedDecimalMultiplyChecked); plan.Decimal.PreferredScale != DecimalPreferredSumScale {
		t.Fatalf("multiply preferred scale = %s", plan.Decimal.PreferredScale)
	}
	if plan := expect("(a / b)", ResolvedDecimalDivideChecked); !plan.Decimal.RoundsInexactQuotient || len(plan.Decimal.PossibleFailures) != 4 {
		t.Fatalf("divide plan = %+v", plan.Decimal)
	}
	if plan := expect("(a % b)", ResolvedDecimalRemainderChecked); !plan.Decimal.TruncatingRemainder {
		t.Fatalf("remainder plan = %+v", plan.Decimal)
	}
	expect("(-a)", ResolvedDecimalNegateChecked)
	if plan := expect("(wide * 2)", ResolvedDecimalMultiplyChecked); plan.Decimal.CoefficientBits != 128 {
		t.Fatalf("decimal128 coefficient bits = %d", plan.Decimal.CoefficientBits)
	}
	if plan := expect("(sum == product)", ResolvedDecimalCompareEQ); plan.RuntimeCheck || !plan.Decimal.ScaleIndependentComparison {
		t.Fatalf("comparison plan = %+v %+v", plan, plan.Decimal)
	}
	constant := func(source, coefficient string, scale int32) {
		t.Helper()
		plan := plans[source].Decimal
		if plan == nil || plan.Constant == nil || plan.Constant.Coefficient.String() != coefficient || plan.Constant.Scale != scale {
			t.Fatalf("%s constant = %+v, want %s scale %d", source, plan, coefficient, scale)
		}
	}
	constant("(1m / 3m)", "3333333333333333333", 19)
	constant("(1m / 8m)", "125", 3)
	constant("(1.5m * 2.50m)", "3750", 3)
	if plan := plans["(third * 3m)"].Decimal; plan == nil || plan.ConstantFailure != "PrecisionExhausted" {
		t.Fatalf("0.3333333333333333333 * 3 needs 19 exact nines: %+v", plan)
	}
}
