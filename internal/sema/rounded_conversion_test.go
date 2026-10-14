package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const roundingPrelude = `module main

enum RoundingMode {
    NearestEven
    NearestAwayFromZero
    TowardZero
    AwayFromZero
    Ceil
    Floor
}

unit rpm physical
impl <rpm> {
    Kind: frequency
    Dimension: [time^-1]
    Scale: 1 / 60
}

unit Hz physical
impl <Hz> {
    Kind: frequency
    Dimension: [time^-1]
    Scale: 1
}

unit m physical
impl <m> {
    Kind: length
    Dimension: [length^1]
    Scale: 1
}
`

func roundedConversionValues(t *testing.T, body string) (*Analyzer, map[string]RoundedConversionPlan, []Error) {
	t.Helper()
	program := parser.New(lexer.NewWithFile(roundingPrelude+body, "rounding.sec")).ParseProgram()
	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	plans := map[string]RoundedConversionPlan{}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || fn.Body == nil {
			continue
		}
		for _, inner := range fn.Body.Statements {
			let, ok := inner.(*ast.LetStatement)
			if !ok {
				continue
			}
			value := let.Value
			if try, isTry := value.(*ast.TryExpression); isTry {
				value = try.Expression
			}
			if plan, found := analyzer.RoundedConversionOf(value); found {
				plans[let.Name.Value] = plan
			}
		}
	}
	return analyzer, plans, errors
}

// Every RoundingMode has its mathematical meaning on positive, negative and
// halfway values for an integral target (MD-047 § 5.6 table), applied once
// to the exact value.
//
// Rules:
//   - rules/types/units.md — "Explicit rounding"
func TestRoundingModesFollowTheirMathematicalDefinitions(t *testing.T) {
	values := []string{"1.6", "-1.6", "2.5", "-2.5"}
	table := map[string][]string{
		"NearestEven":         {"2", "-2", "2", "-2"},
		"NearestAwayFromZero": {"2", "-2", "3", "-3"},
		"TowardZero":          {"1", "-1", "2", "-2"},
		"AwayFromZero":        {"2", "-2", "3", "-3"},
		"Ceil":                {"2", "-1", "3", "-2"},
		"Floor":               {"1", "-2", "2", "-3"},
	}
	var body strings.Builder
	body.WriteString("fn Use() void {\n")
	want := map[string]string{}
	for mode, results := range table {
		for index, value := range values {
			name := "v" + mode + string(rune('A'+index))
			want[name] = results[index]
			body.WriteString("    let " + name + " := int8(" + value + ", RoundingMode." + mode + ")\n")
		}
	}
	body.WriteString("}\n")
	_, plans, errors := roundedConversionValues(t, body.String())
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %+v", errors)
	}
	for name, expected := range want {
		if plan, ok := plans[name]; !ok || plan.Constant == nil || plan.Constant.String() != expected {
			t.Errorf("%s: got %+v, want %s", name, plan.Constant, expected)
		}
	}
}

// A rounded unit conversion applies the exact unit transform first and
// rounds once; a proven integer domain needs no try, a dynamic value is a
// checked conversion with ConversionError, and rounding never excuses an
// out-of-range result or incompatible units.
func TestRoundedUnitConversions(t *testing.T) {
	_, plans, errors := roundedConversionValues(t, `
fn Use(speed: decimal<rpm>, small: uint8) void {
    let hz := try uint16<Hz>(speed, RoundingMode.Ceil) {
        Err(_) => return
    }
    let constant := uint16<Hz>(90<rpm>, RoundingMode.Ceil)
    let third := uint16<Hz>(100<rpm>, RoundingMode.NearestEven)
    let widened := uint16(small, RoundingMode.Floor)
}

fn Bad(speed: decimal<rpm>) void {
    let unprotected := uint16<Hz>(speed, RoundingMode.Ceil)
    let overflow := uint8(300.2, RoundingMode.Floor)
    let length := uint16<Hz>(5<m>, RoundingMode.Ceil)
}
`)
	if plan := plans["hz"]; !plan.Checked || plan.Mode != "Ceil" {
		t.Errorf("dynamic conversion: %+v", plan)
	}
	if plan := plans["constant"]; plan.Constant == nil || plan.Constant.String() != "2" {
		t.Errorf("90 rpm is 1.5 Hz, Ceil 2: %+v", plan)
	}
	if plan := plans["third"]; plan.Constant == nil || plan.Constant.String() != "2" {
		t.Errorf("100 rpm is 5/3 Hz, NearestEven 2: %+v", plan)
	}
	if plan := plans["widened"]; plan.Checked || plan.Minimum == nil || plan.Maximum.String() != "255" {
		t.Errorf("uint8 domain is proven: %+v", plan)
	}
	wants := []string{
		"converting decimal<rpm> to uint16<Hz> may fail with ConversionError.OutOfRange and requires try",
		"300.2 rounds to 300, which is outside uint8",
		"cannot convert incompatible unit quantity decimal<m> to uint16<Hz>",
	}
	if len(errors) != len(wants) {
		t.Fatalf("want %d errors, got %+v", len(wants), errors)
	}
	for _, want := range wants {
		found := false
		for _, err := range errors {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %+v", want, errors)
		}
	}
}
