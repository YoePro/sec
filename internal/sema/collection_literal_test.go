package sema

import (
	"testing"
)

// Empty list literals type as their list type, and list[T] and
// list[T, Capacity] are defaultable independently of element defaultability,
// both for mutable declarations and omitted struct fields.
//
// Rules:
//   - rules/types/default_values.md — "List defaults"
//   - rules/collections/collections.md — §13.3 empty list state
func TestEmptyListLiteralsAndListDefaults(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

type Handle struct {
    raw: RawPtr[byte],
}

type Holder struct {
    handles: list[Handle],
    bounded: list[int, 8],
}

fn F() void {
    let a: list[int] := list[int] {}
    let b := list[int, 8] {}
    let mut c: list[Handle]
    let holder := Holder {}
    discard a
    discard b
    discard c
    discard holder
}
`)
	if len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	for _, name := range []string{"Holder"} {
		if value, kind, ok := DefaultValueDisplay(analyzer.Types()[name]); !ok || kind != StructDefault {
			t.Fatalf("%s default = %q %q %v", name, value, kind, ok)
		}
	}
}

// A capacity mismatch and a zero capacity are rejected; a list default
// displays as its canonical empty literal.
//
// Rules:
//   - rules/collections/collections.md — §13.3 and list capacity
//   - rules/types/default_values.md — "List defaults"
func TestEmptyListLiteralCapacityChecks(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn F() void {
    let a: list[int] := list[int, 4] {}
    let b := list[int, 0] {}
    discard a
    discard b
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want capacity mismatch and zero capacity", errors)
	}
	if value, kind, ok := DefaultValueDisplay(Type{Name: "list", Kind: StructType, TypeArgs: []Type{{Name: "int", Kind: IntType}}}); !ok || kind != CollectionDefault || value != "list[int] {}" {
		t.Fatalf("list default display = %q %q %v", value, kind, ok)
	}
}
