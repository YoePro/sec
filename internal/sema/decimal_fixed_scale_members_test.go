package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeFixedScaleSource(t *testing.T, body string) []string {
	t.Helper()
	source := "module main\n\ntype Amount decimal scale 2 range 0m..1000m\n\n" + body + "\n\nfn main() int {\n    return 0\n}\n"
	p := parser.New(lexer.NewWithFile(source, "fixed.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	var messages []string
	for _, err := range NewAnalyzer().Analyze(program) {
		messages = append(messages, err.Error())
	}
	return messages
}

// MD-049 §§ 12(2), 12(6)–12(7), T43–T47: a fixed-scale named decimal keeps
// the Scale getter, has no Scale setter (with or without try, even for its
// own scale), and inherits Normalize (carrier), Rescale/RoundTo
// (Result[carrier, PrecisionError]) and the ToInt*/ToFloat* conversions,
// never an unchecked T; methods that would construct a T are not inherited.
func TestFixedScaleDecimalMembers(t *testing.T) {
	valid := `fn Scale(amount: Amount) int32 {
    return amount.Scale
}

fn Norm(amount: Amount) decimal {
    return amount.Normalize()
}

fn Res(amount: Amount) Result[decimal, PrecisionError] {
    return amount.Rescale(3)
}

fn Round(amount: Amount) Result[decimal, PrecisionError] {
    return amount.RoundTo(1)
}

fn Whole(amount: Amount) Result[int64, ConversionError] {
    return amount.ToInt64()
}`
	if errs := analyzeFixedScaleSource(t, valid); len(errs) != 0 {
		t.Fatal(errs)
	}
	for name, test := range map[string]struct{ body, want string }{
		"setter with try":     {"fn F(amount: Amount) Result[int32, PrecisionError] {\n    let mut copy := amount\n    try copy.Scale = 3\n    return Ok(0)\n}", "compiler-known property Scale of Amount is read-only"},
		"setter same scale":   {"fn F(amount: Amount) Result[int32, PrecisionError] {\n    let mut copy := amount\n    try copy.Scale = 2\n    return Ok(0)\n}", "compiler-known property Scale of Amount is read-only"},
		"setter without try":  {"fn F(amount: Amount) int32 {\n    let mut copy := amount\n    copy.Scale = 2\n    return 0\n}", "read-only"},
		"carrier is not T":    {"fn F(amount: Amount) Amount {\n    return amount.Normalize()\n}", "Amount"},
		"Floor not inherited": {"fn F(amount: Amount) Amount {\n    return amount.Floor()\n}", "Floor"},
	} {
		t.Run(name, func(t *testing.T) {
			errs := analyzeFixedScaleSource(t, test.body)
			if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), test.want) {
				t.Fatalf("errors %v, want one containing %q", errs, test.want)
			}
		})
	}
}

// MD-049 §§ 12(3)–12(4), 15(4), T41/T42: T op T is T; the plan carries the
// fixed scale S, never rounds a quotient, lists PrecisionExhausted (off the
// grid or beyond the coefficient) and DivisionByZero for / and %, and marks
// T's value contracts for checking. Constants are placed exactly on S.
func TestFixedScaleDecimalOperatorPlans(t *testing.T) {
	source := `module main

type Amount decimal scale 2 range 0m..1000m

let Tiny: Amount := 0.01m
let Two: Amount := 2.00m
let Small := Tiny * Tiny
let Sum := Two + Two
let Exact := Two / Two

fn Divide(a: Amount, b: Amount) Amount {
    return a / b
}

fn main() int {
    return 0
}
`
	p := parser.New(lexer.NewWithFile(source, "fixed.sec"))
	program := p.ParseProgram()
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	plans := map[string]*ResolvedDecimalOperation{}
	for expr, resolved := range a.resolvedOperators {
		if resolved.Decimal != nil {
			plans[expr.String()] = resolved.Decimal
		}
	}
	check := func(spelling string) *ResolvedDecimalOperation {
		plan, ok := plans[spelling]
		if !ok || plan.FixedScale == nil || *plan.FixedScale != 2 || !plan.ContractChecked || plan.RoundsInexactQuotient {
			t.Fatalf("%s: plan %+v", spelling, plan)
		}
		return plan
	}
	if plan := check("(Tiny * Tiny)"); plan.ConstantFailure != "PrecisionExhausted" {
		t.Fatalf("0.01 * 0.01 = %+v, want PrecisionExhausted", plan)
	}
	if plan := check("(Two + Two)"); plan.Constant == nil || plan.Constant.Coefficient.Int64() != 400 || plan.Constant.Scale != 2 {
		t.Fatalf("2.00 + 2.00 = %+v, want (400, 2)", plan.Constant)
	}
	if plan := check("(Two / Two)"); plan.Constant == nil || plan.Constant.Coefficient.Int64() != 100 || plan.Constant.Scale != 2 {
		t.Fatalf("2.00 / 2.00 = %+v, want (100, 2)", plan.Constant)
	}
	if plan := check("(a / b)"); strings.Join(plan.PossibleFailures, ",") != "DivisionByZero,PrecisionExhausted" {
		t.Fatalf("a / b failures %v", plan.PossibleFailures)
	}
}
