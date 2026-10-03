package sema

import (
	"strings"
	"testing"
)

// A generic body may call methods guaranteed by every declared constraint,
// while an unconstrained parameter must not acquire methods from call sites or
// from structural similarity.
//
// Rules:
//   - rules/declarations/generics.md — §14 "Constraint satisfaction"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
func TestGenericConstraintMethodsAreAvailableInGenericBody(t *testing.T) {
	input := `
module main

interface First {
    fn FirstValue() int
}

interface Second {
    fn SecondValue(prefix: string) string
}

fn Valid[T: First & Second](value: T) string {
    let first: int := value.FirstValue()
    return value.SecondValue(try first.ToString() { Err(_) => "" })
}

fn Invalid[T](value: T) int {
    return value.FirstValue()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"unknown function or type value.FirstValue at 18:17",
	})
}

// Receiver capabilities declared by the constraint continue through ordinary
// method-call validation instead of being weakened by generic lookup.
func TestGenericConstraintMethodKeepsMutableReceiverRequirement(t *testing.T) {
	input := `
module main

interface MutableValue {
    mut fn Reset() void
}

fn Valid[T: MutableValue](value: ref mut T) void {
    value.Reset()
}

fn Invalid[T: MutableValue](value: ref T) void {
    value.Reset()
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 {
		t.Fatalf("errors = %v, want one receiver-capability error", errors)
	}
	if !strings.Contains(errors[0].Message, "method Reset requires mutable receiver") {
		t.Fatalf("error = %q, want mutable receiver diagnostic", errors[0].Message)
	}
}
