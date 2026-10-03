package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Contract arguments and explicit defaults are ordinary expressions in a
// SemanticCompileTimeRequiredContext: literals, operators, and immutable
// compile-time-established module bindings establish range bounds, length
// values, divisors, membership values, and defaults.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.1–3.17
func TestContractArgumentsUseSemanticCompileTimeEvaluation(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

let MinimumPort: int := 1
let MaximumPort: int := 0xFFFF
let Scale: float64 := 2.5

type Port int range MinimumPort..MaximumPort
type Upper int range 1..MaximumPort default MinimumPort + 1
type Open int range ..MaximumPort
type Negative int range -MaximumPort..0
type Sized string minLen MinimumPort maxLen MaximumPort
type Stepped int multipleOf MinimumPort + 1
type Picked int in [MinimumPort, MinimumPort + 1]
type Ratio float64 range 0.0..Scale
`)
	assertSemaErrors(t, errors, nil)
	port := analyzer.types["Port"]
	if len(port.Contracts) != 1 {
		t.Fatalf("Port contracts = %+v", port.Contracts)
	}
	bounds := port.Contracts[0].(RangeContract)
	if bounds.Min == nil || bounds.Min.Int64() != 1 || bounds.Max == nil || bounds.Max.Int64() != 0xFFFF {
		t.Fatalf("Port range = %+v, want 1..65535", bounds)
	}
	if upper := analyzer.types["Upper"]; upper.ExplicitDefault == nil || upper.ExplicitDefault.Integer.Int64() != 2 {
		t.Fatalf("Upper default = %+v, want 2", upper.ExplicitDefault)
	}
	ratio := analyzer.types["Ratio"].Contracts[0].(RangeContract)
	if ratio.ExactMax == nil || ratio.ExactMax.FloatString(1) != "2.5" {
		t.Fatalf("Ratio range = %+v, want exact upper bound 2.5", ratio)
	}
}

// A compile-time-known value or default is validated against every contract,
// including argumentless ones, and a default established through a binding is
// validated like a literal default.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.6–3.10
func TestCompileTimeDefaultsAreValidatedAgainstEveryContract(t *testing.T) {
	errors := analyzeSource(t, `
module main

let Odd: int := 3

type EvenValue int even default 4
type EvenBad int even default 3
type EvenFromBinding int even default Odd
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want EvenBad and EvenFromBinding rejected", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.DefaultViolatesContract {
			t.Fatalf("diagnostic = %+v, want S1059", diagnostic)
		}
	}
}

// Runtime-dependent values cannot complete required semantic CTE. Calls and
// property getters are legal semantic CTE but need the not yet implemented
// executor; they receive S1101 instead of a claim that the source is invalid.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.14, 3.17–3.29
func TestContractArgumentsThatCannotBeEstablishedAtCompileTime(t *testing.T) {
	errors := analyzeSource(t, `
module main

let mut Counter: int := 3

fn MaxPort() int {
    return 0xFFFF
}

type Standards struct {}

impl Standards {
    static property MaxPort: int {
        get {
            return 0xFFFF
        }
    }
}

type ByMutable int range 1..Counter
type ByCall int range 1..MaxPort()
type ByProperty int range 1..Standards.MaxPort
type DefaultByCall int default MaxPort()
`)
	want := []struct {
		id      string
		subject string
	}{
		{diagnostics.InvalidContractArgument, "Counter"},
		{diagnostics.SemanticCompileTimeExecutionUnavailable, "MaxPort()"},
		{diagnostics.SemanticCompileTimeExecutionUnavailable, "Standards.MaxPort"},
		{diagnostics.SemanticCompileTimeExecutionUnavailable, "MaxPort()"},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %d", errors, len(want))
	}
	for index, expected := range want {
		if errors[index].ID != expected.id || !strings.Contains(errors[index].Message, expected.subject) {
			t.Fatalf("error %d = %+v, want %s naming %s", index, errors[index], expected.id, expected.subject)
		}
	}
}

// Checked conversion into a constrained named type keeps three layers apart:
// a missing conversion relation is a type error, an intrinsic failure of the
// underlying primitive domain stops before contracts, and declared contracts
// are then checked in source order. Compile-time-known values are diagnosed
// at compile time.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 4.3–4.24
func TestCheckedConversionFailureLayers(t *testing.T) {
	errors := analyzeSource(t, `
module main

type SmallOdd int8 range 1..100 odd

fn Check() void {
    let a := SmallOdd(1000)
    let b := SmallOdd(110)
    let c := SmallOdd(50)
    let d := SmallOdd(51)
    let e: int := int("hello")
    discard a
    discard b
    discard c
    discard d
    discard e
}
`)
	want := []string{
		"value 1000 overflows int8 (the representation of SmallOdd)",
		"value 110 violates range contract SmallOdd 1..100",
		"value 50 violates odd contract SmallOdd",
		"cannot convert string to int",
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %d", errors, len(want))
	}
	for index, message := range want {
		if !strings.Contains(errors[index].Message, message) {
			t.Fatalf("error %d = %q, want %q", index, errors[index].Message, message)
		}
	}
	if errors[1].ID != diagnostics.ValueViolatesContract || errors[2].ID != diagnostics.ValueViolatesContract || errors[0].ID == diagnostics.ValueViolatesContract {
		t.Fatalf("layer identities = %+v, want intrinsic failure distinct from contract failures", errors)
	}
}
