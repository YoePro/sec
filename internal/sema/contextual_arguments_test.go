package sema

import "testing"

// A bare None argument takes its Option[T] type from the parameter in its
// position when every candidate accepting the argument count agrees, for
// ordinary calls and for `new` initializers; disagreeing candidates and a
// visible binding named None give no such context.
//
// Rules:
//   - rules/foundations/language_philosophy.md — §16 "Infer, do not guess"
//   - rules/declarations/impl.md — initializer selection through `new`
func TestBareNoneArgumentUsesAgreedParameterContext(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box struct {
    value: int,
}

impl Box {
    init(first: Option[int], second: int) {
        self.value = second
    }

    init(only: Option[string]) BuildError {
        self.value = 0
    }
}

enum BuildError error {
    Bad,
}

fn Take(first: Option[int], second: int) int {
    return second
}

fn Either(value: Option[int]) void {
}

fn Either(value: Option[string]) void {
}

fn Calls() Result[Box, BuildError] {
    discard Take(None, 1)
    let a := new Box(None, 1)
    discard a
    let b := try new Box(None)
    return Ok(b)
}

fn Ambiguous() void {
    Either(None)
}
`)
	assertSemaErrors(t, errors, []string{
		"undefined variable None at 41:12",
	})
}

// The shared builtin type table is built once for lookups, while every
// Analyzer still receives its own mutable copy.
func TestBuiltinTypeTableCopiesAreIndependent(t *testing.T) {
	first := builtinTypes()
	first["int"] = Type{Name: "changed", Kind: InvalidType}
	if builtinType("int").Kind != IntType || builtinTypes()["int"].Kind != IntType {
		t.Fatal("mutating an Analyzer copy changed the shared builtin type table")
	}
}
