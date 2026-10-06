package sema

import (
	"strings"
	"testing"
)

// unitConversionPlanUnits declares two compatible linear length units with
// the known Kind required for implicit conversion (rules/types/units.md,
// "Known and unknown Kind").
const unitConversionPlanUnits = `
module main

unit Meter physical
unit Milli physical

impl Meter {
	Dimension: [length^1]
	Kind: length
	Scale: 1
}

impl Milli {
	Dimension: [length^1]
	Kind: length
	Scale: 1 / 1000
}
`

func unitConversionPlanAtLine(t *testing.T, analyzer *Analyzer, line int) UnitConversionPlan {
	t.Helper()
	for expr := range analyzer.unitConversionPlans {
		if expressionToken(expr).Line == line {
			plan, ok := analyzer.UnitConversionPlanOf(expr)
			if !ok {
				t.Fatalf("UnitConversionPlanOf(%s) reported no plan", expr.String())
			}
			return plan
		}
	}
	t.Fatalf("no unit conversion plan recorded on line %d", line)
	return UnitConversionPlan{}
}

// Rules: rules/types/units.md — "Exact fixed conversions", "No hidden precision loss".
func TestUnitConversionPlanProvesIntegerValueRanges(t *testing.T) {
	input := unitConversionPlanUnits + `
fn Guarded(a: int64<Meter>) int64<Milli> {
	if a >= -5 && a <= 5 {
		let b: int64<Milli> := a
		return b
	}
	return 0
}

fn GuardedOperand(a: int64<Meter>, b: int64<Milli>) int64<Milli> {
	if a > 0 && a < 100 {
		return b + a
	}
	return b
}

fn ConstantFits() int16<Milli> {
	let a: int16<Meter> := 30
	let b: int16<Milli> := a
	return b
}

fn Take(value: int64<Milli>) int64<Milli> {
	return value
}

fn GuardedArgument(a: int64<Meter>) int64<Milli> {
	if a == 2 {
		return Take(a)
	}
	return 0
}

fn ExactDivision() int64<Meter> {
	let a: int64<Milli> := 4000
	let b: int64<Meter> := a
	return b
}

fn ComparedEitherWay(a: int64<Meter>, b: int64<Milli>) bool {
	if a >= 0 && a <= 10 {
		return a < b
	}
	return false
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	guarded := unitConversionPlanAtLine(t, analyzer, 21)
	if guarded.Proof != UnitConversionIntegerValueRange || guarded.Factor.RatString() != "1000" ||
		guarded.Minimum.String() != "-5000" || guarded.Maximum.String() != "5000" {
		t.Fatalf("guarded plan = %s factor %s range %s..%s", guarded.Proof, guarded.Factor.RatString(), guarded.Minimum, guarded.Maximum)
	}
	if guarded.Source.Named != "Meter" || guarded.Target.Named != "Milli" || guarded.Carrier.Name != "int64" {
		t.Fatalf("guarded plan identities = %s -> %s in %s", guarded.Source.Named, guarded.Target.Named, guarded.Carrier.Name)
	}
	if operand := unitConversionPlanAtLine(t, analyzer, 29); operand.Minimum.String() != "1000" || operand.Maximum.String() != "99000" {
		t.Fatalf("operand plan range = %s..%s", operand.Minimum, operand.Maximum)
	}
	if constant := unitConversionPlanAtLine(t, analyzer, 36); constant.Minimum.String() != "30000" || constant.Maximum.String() != "30000" {
		t.Fatalf("constant plan range = %s..%s", constant.Minimum, constant.Maximum)
	}
	if argument := unitConversionPlanAtLine(t, analyzer, 46); argument.Minimum.String() != "2000" {
		t.Fatalf("argument plan minimum = %s", argument.Minimum)
	}
	if division := unitConversionPlanAtLine(t, analyzer, 53); division.Factor.RatString() != "1/1000" || division.Minimum.String() != "4" {
		t.Fatalf("division plan = factor %s minimum %s", division.Factor.RatString(), division.Minimum)
	}
	if compared := unitConversionPlanAtLine(t, analyzer, 59); compared.Source.Named != "Meter" || compared.Maximum.String() != "10000" {
		t.Fatalf("comparison plan = %s range ..%s", compared.Source.Named, compared.Maximum)
	}
}

// Rules: rules/types/units.md — "No hidden precision loss": overflow and truncation are never hidden.
func TestUnitConversionPlanRejectsUnprovenIntegerConversions(t *testing.T) {
	input := unitConversionPlanUnits + `
fn Unguarded(a: int64<Meter>) int64<Milli> {
	let b: int64<Milli> := a
	return b
}

fn ConstantOverflows() int16<Milli> {
	let a: int16<Meter> := 40
	let b: int16<Milli> := a
	return b
}

fn InexactDivision(a: int64<Milli>) int64<Meter> {
	let b: int64<Meter> := a
	return b
}

fn UnguardedOperand(a: int64<Meter>, b: int64<Milli>) int64<Milli> {
	return b + a
}
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot initialize int64<Milli> with int64<Meter> at 20:25",
		"cannot initialize int16<Milli> with int16<Meter> at 26:25",
		"cannot initialize int64<Meter> with int64<Milli> at 31:25",
		"cannot implicitly convert int64<Meter> to int64<Milli> without loss at 36:11",
	})
	wantHelp := []string{
		"`a` may hold -9223372036854775808..9223372036854775807, which scaled by 1000 is not proven to fit int64<Milli>",
		"`a` holds 40, which scaled by 1000 is not proven to fit int16<Milli>",
		"the factor 1/1000 is exact in int64 only for a single known value it divides",
	}
	for i, want := range wantHelp {
		if !strings.Contains(errors[i].Help, want) {
			t.Fatalf("help %d = %q, want it to contain %q", i, errors[i].Help, want)
		}
	}
}

// A unit annotation never makes a carrier change valid that the plain carriers
// reject.
//
// Rules: rules/types/units.md — "Same named unit", "The unit system does not make an otherwise invalid numeric operation valid".
func TestUnitQuantitiesKeepPlainCarrierRules(t *testing.T) {
	input := unitConversionPlanUnits + `
fn Narrow(a: int64<Meter>) int8<Meter> {
	let b: int8<Meter> := a
	return b
}

fn Widen(a: int8<Meter>) int32<Meter> {
	let b: int32<Meter> := a
	return b
}

fn PlatformSized(a: int64<Meter>, b: int64<Meter>) int<Meter> {
	let c: int<Meter> := a + b
	return c
}

fn FloatWidth(a: float32<Meter>) float64<Meter> {
	let b: float64<Meter> := a
	return b
}

fn MixedWidths(a: int16<Meter>, b: int64<Meter>) int64<Meter> {
	return b + a
}

fn MixedWidthComparison(a: int16<Meter>, b: int64<Meter>) bool {
	return b < a
}
`
	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot initialize int8<Meter> with int64<Meter> at 20:24",
		"cannot initialize int32<Meter> with int8<Meter> at 25:25",
		"cannot initialize int<Meter> with int64<Meter> at 30:25",
		"cannot initialize float64<Meter> with float32<Meter> at 35:27",
		"cannot apply operator + to numeric carriers int64<Meter> and int16<Meter> at 40:11",
		"cannot compare int64<Meter> and int16<Meter> without a lossless unit conversion at 44:11",
	})
}
