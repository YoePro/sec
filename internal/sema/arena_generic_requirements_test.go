package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const arenaGenericSource = `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Point struct {
    x: int,
    y: int,
}

fn Make[T](arena: ref mut Arena) Result[ref mut T, AllocationError] {
    return arena.New[T]()
}

fn Forward[U](arena: ref mut Arena) Result[ref mut U, AllocationError] {
    return Make[U](arena)
}

fn Recurse[V](arena: ref mut Arena, depth: int) Result[ref mut V, AllocationError] {
    if depth > 0 {
        return Recurse[V](arena, depth - 1)
    }
    return arena.New[V]()
}

fn Main() void {
    let mut buffer: byte[8] := [0u, 0u, 0u, 0u, 0u, 0u, 0u, 0u]
    let mut arena := Arena.FromBuffer(buffer)
    let ok := Make[Point](arena)
    discard ok
    let direct := Make[Handle](arena)
    discard direct
    let forwarded := Forward[Handle](arena)
    discard forwarded
    let recursive := Recurse[Point](arena, 2)
    discard recursive
    let recursiveBad := Recurse[Handle](arena, 2)
    discard recursiveBad
    arena.Release()
}
`

// A typed Arena allocation in a generic template is checked for every
// concrete specialization, including specializations reached through
// templates that forward their own parameters, and reported at the concrete
// call with the template allocation as related location. A valid
// specialization and recursive templates produce no extra diagnostics.
//
// Rules:
//   - rules/memory/arena.md — §§19(1–4), 20(2), 21(2), 120(1)
//   - rules/declarations/generics.md — specialization validity
func TestArenaAllocationRequirementsAreCheckedPerSpecialization(t *testing.T) {
	errors := analyzeSourceRaw(t, arenaGenericSource)
	lines := []int{}
	for _, err := range errors {
		if err.ID != diagnostics.ArenaNonTrivialDestructionType ||
			!strings.Contains(err.Message, "Arena.New cannot allocate Handle because it requires destruction in this specialization") ||
			err.RelatedLocationLabel() != "generic Arena allocation" {
			t.Fatalf("unexpected error %+v", err)
		}
		lines = append(lines, err.Line)
	}
	if len(lines) != 3 || lines[0] != 37 || lines[1] != 39 || lines[2] != 43 {
		t.Fatalf("diagnostic lines = %v, want 37 (direct), 39 (forwarded), 43 (recursive)\n%v", lines, errors)
	}
}
