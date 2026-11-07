package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
)

// P18-05: every owning `T[]` value publishes a read-only plan. It is
// MoveOnly and needs destruction whatever T is; its empty default allocates
// nothing and constructs no element; an owner Place has its own storage and
// a separate backing domain. Repeated queries are detached and allocate no
// identity.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 5, 7–11, 24, 55
//   - rules/collections/collections.md — §§ 6.1–6.3
func TestDynamicArrayPlans(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Holder struct {
    values: int[],
    other: int[],
}

fn Locals(param: Handle[]) uint {
    let mut numbers: int[]
    let mut handles: Handle[]
    let holder := Holder { values: <-numbers, other: handles2() }
    return param.Len + holder.values.Len + holder.other.Len + handles.Len
}

fn handles2() int[] {
    let mut empty: int[]
    return <-empty
}

fn Nested(grid: int[][], index: uint) uint {
    return grid[index].Len
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	plans := map[string]ResolvedDynamicArrayPlan{}
	var nodes []ast.Expression
	for expr := range analyzer.resolvedPlaces {
		if plan, ok := analyzer.ResolvedDynamicArrayPlanOf(expr); ok && plan.Owner != nil {
			plans[plan.Owner.Place.String()] = plan
			nodes = append(nodes, expr)
		}
	}
	for _, name := range []string{"param", "holder.values", "holder.other", "handles", "grid[*]"} {
		if _, ok := plans[name]; !ok {
			t.Fatalf("no owner plan for %s; have %v", name, keysOfPlans(plans))
		}
	}
	for name, plan := range plans {
		if plan.CopyClass != CopyMoveOnly || !plan.RequiresDestruction || !plan.EmptyDefault || plan.DefaultAllocates ||
			!plan.MutableDefaultDeclaration || !plan.ImmutableRequiresInitializer || plan.DestructionPlan == "" {
			t.Errorf("%s: type-level facts %+v", name, plan)
		}
	}
	if plan := plans["param"]; plan.ElementCopyClass != CopyMoveOnly || !plan.ElementRequiresDestruction || plan.ElementDestructionPlan == "" {
		t.Errorf("Handle[] element facts %+v", plan)
	}
	// P18-07/08: the empty owner has no backing; destruction destroys
	// elements only when T needs it, then ends the domain and reclaims per
	// the backing's allocation plan.
	if plan := plans["param"]; plan.EmptyBackingRelation != BackingRelationNone || plan.EmptyReclamation != ReclamationNone ||
		plan.BackingAddressStability != AddressStabilityMovable ||
		!reflect.DeepEqual(plan.DestructionTransitions, []BackingTransitionKind{BackingTransitionDestroyElements, BackingTransitionEndDomain, BackingTransitionReclaimPerPlan}) {
		t.Errorf("Handle[] backing facts %+v", plan)
	}
	if plan := plans["holder.values"]; !reflect.DeepEqual(plan.DestructionTransitions, []BackingTransitionKind{BackingTransitionEndDomain, BackingTransitionReclaimPerPlan}) {
		t.Errorf("int[] destruction transitions %v", plan.DestructionTransitions)
	}
	for origin, want := range map[StorageOrigin]ReclamationAuthority{
		StorageOriginArena: ReclamationArena, StorageOriginAllocatorBacked: ReclamationOwningValue, StorageOriginUnknown: ReclamationUnknown,
	} {
		if got := reclamationAuthorityOf(origin); got != want {
			t.Errorf("reclamation authority of %s = %s, want %s", origin, got, want)
		}
	}
	if plan := plans["holder.values"]; plan.ElementCopyClass != CopyTrivial || plan.ElementRequiresDestruction || plan.ElementDestructionPlan != "" {
		t.Errorf("int[] element facts %+v", plan)
	}

	// Distinct owners have distinct backing domains, the backing domain is
	// not the owner's storage, and fields of one struct share its storage.
	values, other := plans["holder.values"].Owner, plans["holder.other"].Owner
	if values.BackingDomain == 0 || other.BackingDomain == 0 || values.BackingDomain == other.BackingDomain {
		t.Errorf("field owners need distinct backing domains: %+v %+v", values, other)
	}
	if values.OwnerStorage == 0 || values.OwnerStorage != other.OwnerStorage {
		t.Errorf("fields of one owner share its storage: %+v %+v", values, other)
	}
	if plans["param"].Owner.BackingDomain == values.BackingDomain {
		t.Errorf("different owners share a backing domain")
	}
	if plans["grid[*]"].Owner.BackingDomain != 0 {
		t.Errorf("a runtime-indexed owner has no static backing identity: %+v", plans["grid[*]"].Owner)
	}

	// Queries are detached and allocate no identity.
	before := analyzer.placeIdentities.clone()
	first, _ := analyzer.ResolvedDynamicArrayPlanOf(nodes[0])
	first.Owner.Place.Root = "mutated"
	again, _ := analyzer.ResolvedDynamicArrayPlanOf(nodes[0])
	if again.Owner.Place.Root == "mutated" {
		t.Fatal("dynamic-array plan aliases analyzer state")
	}
	if !reflect.DeepEqual(before, analyzer.placeIdentities.clone()) {
		t.Fatal("a query allocated an identity")
	}
	if _, ok := ResolveDynamicArrayPlan(builtinTypes()["int"]); ok {
		t.Fatal("a scalar has a dynamic-array plan")
	}
}

func keysOfPlans(plans map[string]ResolvedDynamicArrayPlan) []string {
	keys := []string{}
	for key := range plans {
		keys = append(keys, key)
	}
	return keys
}
