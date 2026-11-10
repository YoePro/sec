package sema

import (
	"fmt"
	"testing"
)

// A generic template that names a later generic template (in a field, a
// union payload, a nested generic argument or a three-level chain) must not
// keep a predeclaration skeleton: after analysis no template or concrete
// instance carries InvalidType field, payload or type-argument metadata, and
// member access through the forward chain resolves.
//
// Rules:
//   - rules/declarations/generics.md — declaration-order independence
//   - rules/compiler/generics_lowering.md — §§ 4, 7–8 concrete storage closure
func TestForwardGenericTemplateReferencesFinalize(t *testing.T) {
	a, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Outer[T] struct {
    inner: Inner[T],
    pair: Pair[T, Later],
    chain: A[T],
    count: int,
}

type Node[T] union {
    Leaf(T),
    Pair(Box[T]),
}

type A[T] struct {
    b: B[T],
}

type B[T] struct {
    c: Inner[T],
}

type Inner[T] struct {
    value: T,
    tag: int,
}

type Pair[L, R] struct {
    left: L,
    right: R,
}

type Box[T] struct {
    left: int,
    item: T,
}

type Later struct {
    x: int,
}

fn Sum(o: Outer[int]) int {
    return o.inner.value + o.pair.left + o.pair.right.x + o.chain.b.c.tag + o.count
}

fn Pick(n: Node[string]) string {
    match n {
        Leaf(v) => return v
        Pair(b) => return b.item
    }
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	var check func(path string, typ Type, depth int)
	check = func(path string, typ Type, depth int) {
		if depth > 6 {
			return
		}
		if typ.Kind == InvalidType {
			t.Fatalf("%s retains InvalidType metadata", path)
		}
		for i, argument := range typ.TypeArgs {
			check(fmt.Sprintf("%s[%d]", path, i), argument, depth+1)
		}
		for _, field := range typ.Fields {
			check(path+"."+field.Name, field.Type, depth+1)
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				check(path+"."+variant.Name, *variant.Payload, depth+1)
			}
			for _, field := range variant.PayloadFields {
				check(path+"."+variant.Name+"."+field.Name, field.Type, depth+1)
			}
		}
	}
	instances := 0
	for key, typ := range a.genericTypeInstances {
		instances++
		check(fmt.Sprintf("instance %v", key), typ, 0)
	}
	seen := 0
	for name, typ := range a.types {
		switch typ.Name {
		case "Outer", "Node", "A", "B", "Inner", "Pair", "Box", "Later":
		default:
			if len(typ.TypeArgs) == 0 {
				continue
			}
		}
		seen++
		check(name, typ, 0)
	}
	if seen < 8 || instances == 0 {
		t.Fatalf("expected declared templates and concrete instances in the type table, saw %d types, %d instances", seen, instances)
	}
}
