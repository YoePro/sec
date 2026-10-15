package temporal_test

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// timeUnits declares the four core time units exactly as core does, plus a
// length unit, and the core RoundingMode, so the fixtures resolve units
// through ordinary unit metadata rather than any fixed list.
const timeUnits = `
unit s physical
impl <s> {
    Kind: time
    Dimension: [time^1]
    Scale: 1
}

unit ms physical
impl <ms> {
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000
}

unit us physical
impl <us> {
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000000
}

unit ns physical
impl <ns> {
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000000000
}

unit m physical
impl <m> {
    Kind: length
    Dimension: [length^1]
    Scale: 1
}

enum RoundingMode {
    NearestEven
    NearestAwayFromZero
    TowardZero
    AwayFromZero
    Ceil
    Floor
}
`

func analyzeDuration(t *testing.T, body string) (*sema.Analyzer, *ast.Program, []sema.Error) {
	t.Helper()
	parsed := parser.New(lexer.NewWithFile("module main\n"+timeUnits+body, "duration.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzer()
	return analyzer, parsed.Program, analyzer.Analyze(parsed.Program)
}

// letValue returns the initializer of the named let in the named function.
func letValue(t *testing.T, program *ast.Program, function, name string) ast.Expression {
	t.Helper()
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || fn.Name.Value != function {
			continue
		}
		for _, inner := range fn.Body.Statements {
			if let, ok := inner.(*ast.LetStatement); ok && let.Name.Value == name {
				return let.Value
			}
		}
	}
	t.Fatalf("no let %s in %s", name, function)
	return nil
}

// Quantity literals are unit-bearing numeric quantities: `5<s>` is decimal<s>
// by the unit's default carrier, an expected carrier with the same unit
// shapes it, a time quantity never silently becomes duration, it takes part
// in ordinary unit algebra, and `5<6`/`5 < s` stay comparisons.
//
// Rules:
//   - rules/types/units.md — "Quantity literals"
//   - rules/foundations/grammar.md — "Quantity literals"
func TestQuantityLiteralsKeepUnitSemantics(t *testing.T) {
	analyzer, program, errors := analyzeDuration(t, `
fn Use(count: int) void {
    let seconds := 5<s>
    let milliseconds := 250<ms>
    let microseconds := 10<us>
    let nanoseconds := 3<ns>
    let negative := -5<s>
    let fractional := 0.5<ns>
    let typed: int64<ms> := 250<ms>
    let velocity := 100<m> / 5<s>
    let tight := 5<6
    let spaced := count < 6
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %+v", errors)
	}
	for name, want := range map[string]string{
		"seconds": "decimal<s>", "milliseconds": "decimal<ms>", "microseconds": "decimal<us>", "nanoseconds": "decimal<ns>",
		"fractional": "decimal<ns>", "typed": "int64<ms>", "tight": "bool", "spaced": "bool",
	} {
		value := letValue(t, program, "Use", name)
		typ, ok := analyzer.ResolvedTypeOf(value)
		if !ok || sema.TypeDisplayName(typ) != want {
			t.Errorf("%s: got %s, want %s", name, sema.TypeDisplayName(typ), want)
		}
	}
	if _, ok := letValue(t, program, "Use", "seconds").(*ast.QuantityLiteral); !ok {
		t.Error("5<s> did not parse as one quantity literal")
	}
	velocity, _ := analyzer.ResolvedTypeOf(letValue(t, program, "Use", "velocity"))
	if velocity.Dimension.Base["length"] != 1 || velocity.Dimension.Base["time"] != -1 {
		t.Errorf("100<m> / 5<s> must have length/time dimension, got %+v", velocity.Dimension)
	}
	if _, converted := analyzer.DurationConversionPlanOf(letValue(t, program, "Use", "seconds")); converted {
		t.Error("a bare time quantity was converted to duration")
	}
}

// Explicit, typed-declaration and call-argument conversions to duration are
// exact: the plan records the exact nanosecond count, int64 boundaries
// succeed, a RoundingMode is applied once to the exact count, an immutable
// quantity binding and a bounded integer domain are proved, and a dynamic
// value needs try duration(...) with ConversionError.
//
// Rules:
//   - rules/types/temporal.md — "Conversion of time quantities to duration"
func TestDurationConversionsAreExact(t *testing.T) {
	analyzer, program, errors := analyzeDuration(t, `
fn Wait(d: duration) void {
}

fn Use(count: int16<ms>, dynamic: decimal<ms>) void {
    let fiveSeconds := duration(5<s>)
    let halfSecond := duration(500<ms>)
    let microsecond := duration(1<us>)
    let nanosecond := duration(1<ns>)
    let negative := duration(-5<s>)
    let zero := duration(0<ns>)
    let declared: duration := 500<ms>
    Wait(250<ms>)
    let elapsed := 5<s>
    let fromBinding := duration(elapsed)
    let ceil := duration(0.5<ns>, RoundingMode.Ceil)
    let floor := duration(-0.5<ns>, RoundingMode.Floor)
    let upper := duration(9223372036854775807<ns>)
    let lower := duration(-9223372036854775808<ns>)
    let bounded := duration(count)
    let checked := try duration(dynamic) {
        Err(_) => return
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %+v", errors)
	}
	for name, want := range map[string]string{
		"fiveSeconds": "5000000000", "halfSecond": "500000000", "microsecond": "1000", "nanosecond": "1",
		"negative": "-5000000000", "zero": "0", "declared": "500000000", "fromBinding": "5000000000",
		"ceil": "1", "floor": "-1", "upper": "9223372036854775807", "lower": "-9223372036854775808",
	} {
		plan, ok := analyzer.DurationConversionPlanOf(letValue(t, program, "Use", name))
		if !ok || plan.Proof != sema.DurationConversionConstant || plan.Nanoseconds.String() != want {
			t.Errorf("%s: plan %+v, want constant %s ns", name, plan, want)
		}
	}
	if plan, ok := analyzer.DurationConversionPlanOf(letValue(t, program, "Use", "bounded")); !ok || plan.Proof != sema.DurationConversionBoundedDomain ||
		plan.Minimum.String() != "-32768000000" || plan.Maximum.String() != "32767000000" {
		t.Errorf("bounded int16<ms> domain: %+v", plan)
	}
	try, _ := letValue(t, program, "Use", "checked").(*ast.TryExpression)
	if try == nil {
		t.Fatal("checked conversion is not a try")
	}
	if plan, ok := analyzer.DurationConversionPlanOf(try.Expression); !ok || plan.Proof != sema.DurationConversionChecked || plan.Failure != sema.ConversionFailurePrecisionLoss {
		t.Errorf("dynamic conversion: %+v", plan)
	}
}

// Every invalid conversion is a precise compile-time S1153 error: inexact
// without a rounding mode, one nanosecond beyond either int64 boundary, a
// non-time dimension, an unprovable value in a duration context; an
// unprotected dynamic explicit conversion requires try.
func TestInvalidDurationConversionsAreRejected(t *testing.T) {
	_, _, errors := analyzeDuration(t, `
fn Wait(d: duration) void {
}

fn Bad(dynamic: decimal<ms>, wide: int64<s>) void {
    let half := duration(0.5<ns>)
    let above := duration(9223372036854775808<ns>)
    let below := duration(-9223372036854775809<ns>)
    let length := duration(100<m>)
    let context: duration := dynamic
    Wait(wide)
    let unprotected := duration(dynamic)
}
`)
	want := []string{
		"it is 1/2 nanoseconds, not a whole number",
		"9223372036854775808 nanoseconds is outside the signed 64-bit duration range",
		"-9223372036854775809 nanoseconds is outside the signed 64-bit duration range",
		"decimal<m> does not have the dimension of time",
		"cannot initialize duration with decimal<ms>: an already typed quantity is not converted to duration implicitly",
		"argument 1 to Wait must be duration, got int64<s>: an already typed quantity is not converted to duration implicitly",
		"converting decimal<ms> to duration may fail with ConversionError.PrecisionLoss and requires try",
	}
	if len(errors) != len(want) {
		t.Fatalf("want %d errors, got %+v", len(want), errors)
	}
	for _, fragment := range want {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, fragment) {
				found = true
				if !strings.Contains(fragment, "requires try") && err.ID != "S1153" {
					t.Errorf("%q has ID %q, want S1153", fragment, err.ID)
				}
			}
		}
		if !found {
			t.Errorf("missing %q in %+v", fragment, errors)
		}
	}
}

// All six rounding modes follow their mathematical definitions on positive,
// negative, integral and halfway values, applied once to the exact count.
func TestDurationRoundingModes(t *testing.T) {
	cases := map[string][]string{
		// value:          NearestEven NearestAwayFromZero TowardZero AwayFromZero Ceil Floor
		"2.5<ns>":  {"2", "3", "2", "3", "3", "2"},
		"-2.5<ns>": {"-2", "-3", "-2", "-3", "-2", "-3"},
		"3.5<ns>":  {"4", "4", "3", "4", "4", "3"},
		"2.4<ns>":  {"2", "2", "2", "3", "3", "2"},
		"-2.6<ns>": {"-3", "-3", "-2", "-3", "-2", "-3"},
		"7<ns>":    {"7", "7", "7", "7", "7", "7"},
		"1.5<us>":  {"1500", "1500", "1500", "1500", "1500", "1500"},
	}
	modes := []string{"NearestEven", "NearestAwayFromZero", "TowardZero", "AwayFromZero", "Ceil", "Floor"}
	var body strings.Builder
	body.WriteString("fn Use() void {\n")
	index := 0
	names := map[string][2]string{}
	for value := range cases {
		for position, mode := range modes {
			name := "v" + big.NewInt(int64(index)).String()
			index++
			names[name] = [2]string{value, cases[value][position]}
			body.WriteString("    let " + name + " := duration(" + value + ", RoundingMode." + mode + ")\n")
		}
	}
	body.WriteString("}\n")
	analyzer, program, errors := analyzeDuration(t, body.String())
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %+v", errors)
	}
	for name, expectation := range names {
		plan, ok := analyzer.DurationConversionPlanOf(letValue(t, program, "Use", name))
		if !ok || plan.Nanoseconds.String() != expectation[1] {
			t.Errorf("%s (%s, %s): got %+v, want %s", name, expectation[0], plan.Rounding, plan.Nanoseconds, expectation[1])
		}
	}
}

// MD-015 (2026-10-15): only a quantity *literal* materializes as duration in
// an expected-duration position. A binding already typed as a unit-bearing
// quantity — declared or inferred — is never converted implicitly, even when
// its value is provable; it needs a written duration(...). A duration value
// passes as itself.
//
// Rules:
//   - rules/types/temporal.md — § 5.5
func TestTypedQuantitiesNeedAWrittenDurationConversion(t *testing.T) {
	_, _, errors := analyzeDuration(t, `
fn Wait(d: duration) void {
}

fn Use() void {
    Wait(5<s>)
    Wait(500<ms>)
    Wait(-5<s>)

    let typed: int64<s> := 5
    Wait(typed)

    let inferred := 5<s>
    Wait(inferred)

    let converted := duration(5<s>)
    Wait(converted)

    let explicit := duration(typed)
    Wait(explicit)

    let declared: duration := inferred
}
`)
	want := map[string]bool{
		"argument 1 to Wait must be duration, got int64<s>: an already typed quantity":   false,
		"argument 1 to Wait must be duration, got decimal<s>: an already typed quantity": false,
		"cannot initialize duration with decimal<s>: an already typed quantity":          false,
	}
	if len(errors) != len(want) {
		t.Fatalf("want %d errors, got %+v", len(want), errors)
	}
	for _, err := range errors {
		for fragment := range want {
			if strings.Contains(err.Message, fragment) {
				want[fragment] = true
				if err.ID != "S1153" || !strings.Contains(err.Message, "Write duration(") {
					t.Errorf("%q lacks S1153 or the written-conversion advice: %+v", fragment, err)
				}
			}
		}
	}
	for fragment, found := range want {
		if !found {
			t.Errorf("missing %q in %+v", fragment, errors)
		}
	}
}
