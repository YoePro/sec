package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// Every default failure keeps the stable identity named by the default-values
// rulebook instead of a generic invalid-default or no-default diagnostic.
//
// Rules:
//   - rules/types/default_values.md — "Explicit type defaults"
//   - rules/types/default_values.md — "Ambiguous nearest-to-zero values"
//   - rules/types/default_values.md — "Diagnostics"
//   - rules/types/contracts.md — "Explicit defaults" and "Diagnostics"
func TestDefaultDiagnosticsUseStableIdentities(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/default_diagnostics_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	want := []struct {
		id     string
		line   int
		column int
	}{
		{diagnostics.DefaultNotRepresentable, 7, 26},
		{diagnostics.DefaultNotRepresentable, 8, 23},
		{diagnostics.DefaultViolatesContract, 11, 38},
		{diagnostics.AmbiguousImplicitDefault, 29, 13},
		{diagnostics.InvalidDefaultedField, 31, 19},
		{diagnostics.AmbiguousImplicitDefault, 31, 19},
		{diagnostics.AmbiguousImplicitDefault, 33, 19},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %d", errors, len(want))
	}
	for index, expected := range want {
		got := errors[index]
		if got.ID != expected.id || got.Line != expected.line || got.Column != expected.column || got.Help == "" {
			t.Fatalf("error %d = %+v, want %s at %d:%d with help", index, got, expected.id, expected.line, expected.column)
		}
	}
	if errors[3].Help != "provide an explicit initializer or declare an explicit default on NonZeroOdd; -1 and 1 are equally near zero" {
		t.Fatalf("ambiguous help = %q", errors[3].Help)
	}
}

// A union payload field whose type has no default (without a nearest-value
// tie) reports types.no-default-value.
//
// Rule: rules/types/default_values.md — "Diagnostics" (types.no-default-value).
func TestUnionPayloadWithoutDefaultUsesNoDefaultValue(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type NonZeroOdd int range -9..9 odd

type Inner struct {
    candidate: NonZeroOdd,
}

type Figure union {
    Point {
        inner: Inner,
    },
}

fn main() void {
    let figure := Figure.Point {}
    discard figure
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.TypeNoDefaultValue {
		t.Fatalf("errors = %v, want one S1062", errors)
	}
}

// The explicit-default checks are ordered: representability before contract
// satisfaction, and the first violated contract in source order is named.
//
// Rules:
//   - rules/types/default_values.md — "Explicit type defaults"
//   - rules/types/contracts.md — "Composition"
func TestExplicitDefaultNamesFirstViolatedContract(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Step int range 0..100 multipleOf 5 even default 15
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.DefaultViolatesContract {
		t.Fatalf("errors = %v, want one S1059", errors)
	}
	if errors[0].Help != "Step requires even; choose a value satisfying every type contract" {
		t.Fatalf("help = %q", errors[0].Help)
	}
}

// Contract failure families carry stable registered identities instead of
// ad-hoc messages.
//
// Rules:
//   - rules/types/contracts.md — "Applicability", "Composition", "Ordered membership", "Initialization and assignment", "Diagnostics"
//   - rules/tooling/diagnostics.md — § 2(6) stable registered identity
func TestContractDiagnosticsUseStableIdentities(t *testing.T) {
	source, err := os.ReadFile("../../testdata/contracts/contract_diagnostic_ids_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	want := []string{
		diagnostics.InapplicableContract,
		diagnostics.UnsatisfiableContractSet,
		diagnostics.InvalidContractArgument,
		diagnostics.IncompatibleMembershipValue,
		diagnostics.ValueViolatesContract,
		diagnostics.ValueViolatesContract,
		diagnostics.ConstrainedAssignmentRequiresTry,
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %d", errors, len(want))
	}
	for index, id := range want {
		if errors[index].ID != id || errors[index].Help == "" {
			t.Fatalf("error %d = %+v, want %s with help", index, errors[index], id)
		}
	}
}

// A multipleOf divisor is an ordinary expression in a
// SemanticCompileTimeRequiredContext (MD-011): an immutable compile-time
// binding establishes it, while a mutable binding cannot, and the
// unestablished divisor never produces an empty semantic contract.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.4, 3.15–3.17
//   - rules/types/contracts.md — "Integer contracts"
func TestMultipleOfDivisorUsesSemanticCompileTimeEvaluation(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

let Step: int := 5
type Stepped int multipleOf Step * 2
`)
	assertSemaErrors(t, errors, nil)
	found := false
	for _, contract := range analyzer.types["Stepped"].Contracts {
		if multiple, ok := contract.(MultipleOfContract); ok && multiple.Value.Int64() == 10 {
			found = true
		}
	}
	if !found {
		t.Fatalf("contracts = %+v, want multipleOf 10", analyzer.types["Stepped"].Contracts)
	}

	analyzer, errors = analyzeSourceWithAnalyzerRaw(t, `module main

let mut Step: int := 5
type Stepped int multipleOf Step
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.InvalidContractArgument || errors[0].Message != "multipleOf contract divisor must be a compile-time integer" {
		t.Fatalf("errors = %v, want one S1066 divisor diagnostic", errors)
	}
	for _, contract := range analyzer.types["Stepped"].Contracts {
		if _, ok := contract.(MultipleOfContract); ok {
			t.Fatal("unestablished multipleOf divisor produced an empty semantic contract")
		}
	}
}

// Enum values support compile-time semantic equality, so enum-based named
// types accept ordered enum-member membership, reject duplicates and unknown
// members, default to the first listed member, and prove member values.
//
// Rules:
//   - rules/types/contracts.md — "Applicability" and "Ordered membership"
//   - rules/types/default_values.md — "in [...] constrained types"
func TestEnumMembershipContracts(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

enum Color {
    Red,
    Green,
    Blue,
}
type Primary Color in [Color.Blue, Color.Red]
type Duplicate Color in [Color.Red, Color.Red]
type Unknown Color in [Color.Purple]

fn F() void {
    let ok: Primary := Primary.Red
    let bad: Primary := Primary.Green
    let mut defaulted: Primary
    discard ok
    discard bad
    discard defaulted
}
`)
	want := []string{diagnostics.DuplicateContractMembershipValue, diagnostics.InvalidContractArgument, diagnostics.ValueViolatesContract}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, id := range want {
		if errors[index].ID != id {
			t.Fatalf("error %d = %+v, want %s", index, errors[index], id)
		}
	}
	if value, kind, ok := DefaultValueDisplay(analyzer.types["Primary"]); !ok || kind != MembershipDefault || value != "Color.Blue" {
		t.Fatalf("Primary default = %q %q %v", value, kind, ok)
	}
}

// Membership values inherited from a base type are revalidated against every
// newly added derived contract and are never silently filtered.
//
// Rule: rules/types/contracts.md — "Ordered membership" and "Composition".
func TestInheritedMembershipRevalidatedAgainstDerivedContracts(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Short string in ["a", "bb"]
type Longer Short minLen 2
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.InvalidMembershipValue || errors[0].Message != `membership value "a" violates another contract on Longer` {
		t.Fatalf("errors = %v, want inherited \"a\" rejected", errors)
	}
}
