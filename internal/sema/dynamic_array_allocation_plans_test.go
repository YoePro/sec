package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// P18-06: every Append on an owning `T[]` publishes an allocation plan in
// the site's canonical allocation context, with public CollectionError and
// allocation-layer AllocationError. A range append is one operation with its
// exact count; an empty range allocates nothing. The receiver's backing
// domain is the one its dynamic-array plan names. A list append is not a
// dynamic-array plan.
//
// P18-07/08: a possibly allocating Append has its own allocation identity,
// an Owned backing relation and the context's reclamation authority (Arena in
// hosted, so no per-array reclamation plan), and names its backing
// transitions: establish when absent, none within capacity, advance-epoch on
// growth, none on failure. An empty range creates no identity.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 15–31, 56–57
//   - rules/collections/collections.md — §§ 6.4, 6.7
func TestDynamicArrayAllocationPlans(t *testing.T) {
	const source = `module main

fn Fill() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    try values.Append(1..9)
    try values.Append(1..<1)
    let mut items: list[int]
    try items.Append(2)
    return Ok()
}
`
	for _, profile := range []string{"hosted", "noalloc"} {
		parsed := parser.New(lexer.New(source))
		program := parsed.ParseProgram()
		if len(parsed.Errors()) > 0 {
			t.Fatal(parsed.Errors())
		}
		analyzer := NewAnalyzer()
		analyzer.SetAllocationProfile(profile)
		if errors := analyzer.Analyze(program); len(errors) != 0 {
			t.Fatalf("%s: %v", profile, errors)
		}
		var calls []*ast.CallExpression
		walkASTValue(reflect.ValueOf(program), func(node any) {
			if call, ok := node.(*ast.CallExpression); ok {
				if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil && member.Property.Value == "Append" {
					calls = append(calls, call)
				}
			}
		})
		if len(calls) != 4 {
			t.Fatalf("found %d Append calls", len(calls))
		}
		want := []struct {
			operation AllocationOperationKind
			count     string
			allocates bool
		}{{AllocationDynamicArrayAppend, "1", true}, {AllocationDynamicArrayAppendRange, "9", true}, {AllocationDynamicArrayAppendRange, "0", false}}
		for index, expected := range want {
			plan, ok := analyzer.ResolvedAllocationPlanOf(calls[index])
			if !ok {
				t.Fatalf("%s: Append %d has no plan", profile, index)
			}
			if plan.Operation != expected.operation || plan.Count == nil || plan.Count.String() != expected.count || plan.MayAllocate != expected.allocates {
				t.Errorf("%s: Append %d plan %+v", profile, index, plan)
			}
			if !plan.Fallible || plan.FailureType.Name != "CollectionError" || plan.AllocationFailureType.Name != "AllocationError" || plan.ElementType.Kind != IntType {
				t.Errorf("%s: Append %d failure/type facts %+v", profile, index, plan)
			}
			if plan.Context.Profile != profile || plan.Context.Available != (profile == "hosted") ||
				profile == "hosted" && plan.StorageOrigin != StorageOriginArena {
				t.Errorf("%s: Append %d context %+v origin %s", profile, index, plan.Context, plan.StorageOrigin)
			}
			receiver := calls[index].Callee.(*ast.MemberExpression).Object
			array, _ := analyzer.ResolvedDynamicArrayPlanOf(receiver)
			if plan.Receiver == nil || array.Owner == nil || plan.Receiver.BackingDomain == 0 || plan.Receiver.BackingDomain != array.Owner.BackingDomain {
				t.Errorf("%s: Append %d receiver %+v, array owner %+v", profile, index, plan.Receiver, array.Owner)
			}
		}
		if _, ok := analyzer.ResolvedAllocationPlanOf(calls[3]); ok {
			t.Errorf("%s: a list append has a dynamic-array allocation plan", profile)
		}
		// P18-07/08: identities, relations and the backing transition.
		allocations := map[AllocationIdentityID]bool{}
		for index := 0; index < 3; index++ {
			plan, _ := analyzer.ResolvedAllocationPlanOf(calls[index])
			if plan.ContextIdentity == 0 {
				t.Errorf("%s: Append %d has no allocation context identity", profile, index)
			}
			if !plan.MayAllocate {
				if plan.Allocation != 0 || plan.BackingRelation != "" || plan.ReclamationPlan != 0 ||
					plan.Transition.WhenAbsent != BackingTransitionNone || plan.Transition.Growth != BackingTransitionNone {
					t.Errorf("%s: an empty range has backing facts %+v", profile, plan)
				}
				continue
			}
			if plan.Allocation == 0 || allocations[plan.Allocation] {
				t.Errorf("%s: Append %d allocation identity %s is missing or shared", profile, index, plan.Allocation)
			}
			allocations[plan.Allocation] = true
			wantAuthority := ReclamationUnknown
			if profile == "hosted" {
				wantAuthority = ReclamationArena
			}
			if plan.BackingRelation != BackingRelationOwned || plan.ReclamationAuthority != wantAuthority || plan.ReclamationPlan != 0 ||
				plan.AddressStability != AddressStabilityMovable || plan.MemorySpace != AddressSpaceOrdinary {
				t.Errorf("%s: Append %d relations %+v", profile, index, plan)
			}
			transition := plan.Transition
			if transition.Domain == 0 || transition.Domain != plan.Receiver.BackingDomain ||
				transition.WhenAbsent != BackingTransitionEstablishDomain || transition.WithinCapacity != BackingTransitionNone ||
				transition.Growth != BackingTransitionAdvanceEpoch || transition.OnFailure != BackingTransitionNone ||
				transition.ReclaimsRelocatedBacking || !transition.InvalidatesElementReferences {
				t.Errorf("%s: Append %d transition %+v", profile, index, transition)
			}
		}
		first, _ := analyzer.ResolvedAllocationPlanOf(calls[0])
		first.Count.SetInt64(99)
		if again, _ := analyzer.ResolvedAllocationPlanOf(calls[0]); again.Count.Int64() != 1 {
			t.Fatal("allocation plans alias analyzer state")
		}
	}
}
