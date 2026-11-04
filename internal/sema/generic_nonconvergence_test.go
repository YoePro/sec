package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A template that calls itself with a parameter replaced by a type properly
// containing it demands ever larger specializations, so Sema reports
// non-convergence at the call and terminates (BUG-000023). Finite cycles with
// unchanged or permuted arguments, including mutual recursion, stay valid.
//
// Rules: rules/compiler/monomorphization.md — §§ 18.3, 18.5, 18.7.
func TestGenericSelfRecursionWithGrowingArgumentIsRejected(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box[T] struct {
    Value: T,
}

fn Grow[T](value: T, n: int) int {
    if n <= 0 {
        return 0
    }
    return Grow(Box[T] { Value: value }, n - 1)
}

fn Same[T](value: T, n: int) int {
    if n <= 0 {
        return 0
    }
    return Same(<-value, n - 1)
}

fn Swap[A, B](left: A, right: B, n: int) int {
    if n <= 0 {
        return 0
    }
    return Swap(<-right, <-left, n - 1)
}

fn First[T](value: T, n: int) int {
    if n <= 0 {
        return 0
    }
    return Second(<-value, n - 1)
}

fn Second[T](value: T, n: int) int {
    return First(<-value, n)
}

fn main() int {
    return Grow(1, 3) + Same(1, 2) + Swap(1, true, 2) + First(1, 2)
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.NonConvergingSpecialization ||
		!strings.Contains(errors[0].Message, "specializes T as Box[T]") || errors[0].Line != 12 {
		t.Fatalf("errors = %v, want one S1198 at the Grow recursion", errors)
	}
}
