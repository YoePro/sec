package sema

import "testing"

// A defaultable generic instance materializes its default with its exact
// instantiated type; the synthesized literal is never re-resolved from the
// bare generic declaration name. An unconstrained T stays NonDefaultable.
//
// Rules:
//   - rules/types/default_values.md — struct and union defaultability, generic parameters are not assumed Defaultable
func TestGenericInstanceDefaultsUseExactInstantiatedType(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box[T] struct {
    value: T,
}

type Pair struct {
    box: Box[int],
}

type Cell[T] union {
    Unset { inner: Box[int] } default
    Set(T)
}

type Slot[T] union {
    Empty default
    Full(T)
}

fn Make[T]() void {
    let mut generic: T
}

fn Defaults() void {
    let mut box: Box[int]
    let mut nested: Box[int[2]]
    let mut pair: Pair
    let mut cell: Cell[int]
    let mut slot: Slot[int]
    let mut callable: Box[fn() void]
}
`)
	assertSemaErrors(t, errors, []string{
		"mutable variable generic of type T requires an initializer because the type has no default value at 23:13",
		"mutable variable callable of type Box[fn() void] requires an initializer because the type has no default value at 32:13",
	})
}
