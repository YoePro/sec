package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// @noAlloc is a compiler-verified transitive guarantee: a direct or
// transitive allocation, fallible allocation under try, a call with unknown
// allocation behavior, or an extern call without a trusted @noAlloc contract
// violates it with S1108; allocation-free bodies and trusted foreign
// contracts satisfy it.
//
// Rules:
//   - rules/foundations/attributes.md — "@noAlloc", "@noAlloc verification", "@noAlloc and fallible allocation", "Sec code versus foreign declarations"
//   - rules/memory/allocation.md — § 24 "Allocation effects"
func TestNoAllocGuaranteeIsVerifiedTransitively(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantPath string
		wantHelp string
	}{
		{name: "direct arena allocation", wantPath: "via Fill", wantHelp: "acquires new storage", source: `
@noAlloc
fn Fill(buffer: ref mut byte[]) void {
	let mut arena := Arena.FromBuffer(buffer)
	let value := arena.New[int]()
	arena.Release()
}`},
		{name: "transitive allocation", wantPath: "via Outer -> Inner", wantHelp: "It comes from Inner", source: `
fn Inner(buffer: ref mut byte[]) void {
	let mut arena := Arena.FromBuffer(buffer)
	let values := arena.Alloc[byte](4u)
	arena.Release()
}

@noAlloc
fn Outer(buffer: ref mut byte[]) void {
	Inner(buffer)
}`},
		{name: "string concatenation under try", wantPath: "via Join", wantHelp: "try only handles its failure", source: `
@noAlloc
fn Join(left: string, right: string) Result[string, StringError] {
	let joined := try left + right
	return Ok(joined)
}`},
		{name: "unknown function value", wantPath: "unknown allocation behavior via Apply", wantHelp: "not known here", source: `
@noAlloc
fn Apply(operation: fn(int) int, value: int) int {
	return operation(value)
}`},
		{name: "untrusted extern call", wantPath: "unknown allocation behavior via Call", wantHelp: "trusted foreign contract", source: `
extern "C" fn Native(value: int32) int32

@noAlloc
fn Call(value: int32) int32 {
	let mut result: int32 := 0
	unsafe {
		result = Native(value)
	}
	return result
}`},
		{name: "trusted extern call", source: `
@noAlloc
extern "C" fn Native(value: int32) int32

@noAlloc
fn Call(value: int32) int32 {
	let mut result: int32 := 0
	unsafe {
		result = Native(value)
	}
	return result
}`},
		{name: "allocation free body", source: `
fn Twice(value: int) int {
	return value * 2
}

@noAlloc
fn Compute(values: ref int[4], value: int) int {
	let copied := value
	let view := ref values
	return Twice(copied)
}`},
		{name: "allocation only in an uncalled function", source: `
fn Unused(left: string, right: string) Result[string, StringError] {
	let joined := try left + right
	return Ok(joined)
}

@noAlloc
fn Compute(value: int) int {
	return value + 1
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.source+"\n")
			var violations []Error
			for _, err := range errors {
				if err.ID == diagnostics.NoAllocViolation {
					violations = append(violations, err)
				}
			}
			if test.wantPath == "" {
				if len(errors) != 0 {
					t.Fatalf("errors = %v, want none", errors)
				}
				return
			}
			if len(violations) != 1 {
				t.Fatalf("errors = %v, want one S1108", errors)
			}
			if !strings.Contains(violations[0].Message, test.wantPath) || !strings.Contains(violations[0].Help, test.wantHelp) {
				t.Fatalf("S1108 = %q, help %q; want %q and %q", violations[0].Message, violations[0].Help, test.wantPath, test.wantHelp)
			}
		})
	}
}

// Copy, assignment, move initialization and assignment, parameter passing,
// return, reference and slice-view creation, and control-flow joins,
// matches, and discards never allocate: a @noAlloc function made of them
// verifies and records no allocation site.
//
// Rules:
//   - rules/memory/allocation.md — § 4 "Operations that must not silently allocate", § 30 "Required test families"
func TestOrdinaryOperationsDoNotAllocate(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

@noCopy
type Token struct {
	id: int,
}

type Pair struct {
	left: int,
	right: int,
}

fn Inspect(values: ref int[4]) int {
	return values[0]
}

fn Consume(token: Token) int {
	return token.id
}

fn Pick(pair: Pair, flag: bool, maybe: Option[int]) Pair {
	let mut copy := pair
	copy = pair
	if flag {
		copy.left = 1
	} else {
		copy.right = 2
	}
	match maybe {
		Some(value) => {
			copy.left = value
		}
		None => {
			discard flag
		}
	}
	return copy
}

@noAlloc
fn Ordinary(values: ref int[4], pair: Pair, flag: bool, maybe: Option[int], first: Token) int {
	let total := Inspect(values)
	let window := ref values[0..2]
	let chosen := Pick(pair, flag, maybe)
	let second :<- first
	return total + chosen.left + window[0] + Consume(<-second)
}
`)
	for _, err := range errors {
		t.Errorf("unexpected error: %v", err)
	}
	graph := analyzer.CallGraph()
	for _, name := range []string{"Ordinary", "Pick", "Inspect", "Consume"} {
		summary := graph.ArenaSummary(callGraphNodeIDByName(t, graph, name))
		if len(summary.DirectEffects) != 0 || summary.MayAllocate || summary.AllocationUnknown {
			t.Errorf("%s allocation summary = %+v, want allocation-free", name, summary)
		}
	}
}
