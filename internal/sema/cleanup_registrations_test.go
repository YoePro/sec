package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Initializers publish success-continuation registrations from canonical
// binding/Place and destruction facts, without giving borrowed values ownership.
// Rules: rules/memory/destruction.md — §6.1(1);
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§27,34,38.
func TestOwnedCleanupInitializationPlans(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_initializations.sec")
	seen := map[string]int{}
	actions := map[CleanupActionID]bool{}
	for stmt := range a.ownedCleanupInitializations {
		plan, ok := a.ResolvedOwnedCleanupInitializationOf(stmt)
		if !ok || plan.Action == 0 || plan.Scope == 0 || plan.Binding == 0 || plan.Place.RootID == 0 || plan.Invocation == "" || plan.Activation != CleanupAfterInitialization || actions[plan.Action] {
			t.Fatalf("invalid registration: %+v", plan)
		}
		actions[plan.Action] = true
		binding, ok := a.ResolvedBindingDeclaredAt(stmt.Name.Token.File, stmt.Name.Token.Line, stmt.Name.Token.Column)
		if !ok || binding.ID != plan.Binding {
			t.Fatal("registration lost canonical binding identity")
		}
		root, ok := plan.Destruction.Plan(plan.Destruction.Root)
		if !ok || root.Kind == DestructionTrivial {
			t.Fatal("non-trivial registration has no destruction obligation")
		}
		name := a.callGraph.nodes[plan.Invocation].Name
		seen[name]++
		switch name {
		case "Dynamic":
			if root.Kind != DestructionUnsupported || root.Unsupported == "" {
				t.Fatal("P18 boundary was silently classified as supported destruction")
			}
		case "Array":
			if root.Kind != DestructionFixedArray || root.ElementCount != "2" || root.Element == "" {
				t.Fatal("array registration lost its compact exact destruction plan")
			}
		case "Aggregate":
			if root.Kind != DestructionStruct || len(root.Fields) != 2 || root.Fields[0].Field != "second" || root.Fields[1].Field != "first" {
				t.Fatal("aggregate registration lost its reverse-field destruction plan")
			}
		case "Fallible":
			if _, ok := stmt.Value.(*ast.TryExpression); !ok {
				t.Fatal("fallible initialization fixture did not reach the success-only producer")
			}
		default:
			if root.Kind != DestructionStruct || !root.CustomFree {
				t.Fatal("custom-free obligation was dropped")
			}
		}
	}
	want := map[string]int{"Inferred": 1, "Typed": 1, "Array": 1, "Aggregate": 1, "Fallible": 1, "Moved": 2, "Dynamic": 1}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("registrations = %v, want %v (borrowed/trivial locals must be absent)", seen, want)
	}
}

// Read-only facts detach the complete destruction graph and nested Place
// types. Reanalysis reproduces identities and drops obsolete source nodes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§38,79–80.
func TestOwnedCleanupInitializationsDetachedAndReset(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_initializations.sec")
	reference, _ := analyzeDeferPlanFixture(t, "cleanup_initializations.sec")
	want := map[CleanupActionID]ResolvedOwnedCleanupRegistration{}
	for _, plan := range reference.ownedCleanupInitializations {
		want[plan.Action] = plan
	}
	beforeTypes, beforeActions := len(a.expressionTypes), len(a.cleanupIdentities.actions)
	for stmt, stored := range a.ownedCleanupInitializations {
		plan, ok := a.ResolvedOwnedCleanupInitializationOf(stmt)
		if !ok {
			t.Fatal("missing registration query")
		}
		plan.Place.Root = "changed"
		if plan.Place.Type.Element != nil {
			plan.Place.Type.Element.Name = "changed"
		}
		for id, destruction := range plan.Destruction.Plans {
			if len(destruction.Fields) != 0 {
				destruction.Fields[0].Field = "changed"
			}
			destruction.CustomFree = false
			plan.Destruction.Plans[id] = destruction
			delete(plan.Destruction.Plans, id)
		}
		again, _ := a.ResolvedOwnedCleanupInitializationOf(stmt)
		if !reflect.DeepEqual(again, want[stored.Action]) {
			t.Fatal("query mutated analyzer-owned registration facts")
		}
	}
	if _, ok := a.ResolvedOwnedCleanupInitializationOf(&ast.LetStatement{}); ok {
		t.Fatal("unknown declaration acquired a registration")
	}
	if beforeTypes != len(a.expressionTypes) || beforeActions != len(a.cleanupIdentities.actions) {
		t.Fatal("query inferred or allocated identities")
	}
	old := a.ownedCleanupInitializations
	source, err := os.ReadFile("../../testdata/sema/cleanup_initializations_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if errors := a.Analyze(program); len(errors) != 2 {
		t.Fatalf("expected wrong-type and implicit-copy rejection: %v", errors)
	}
	if len(a.ownedCleanupInitializations) != 1 {
		t.Fatal("invalid initialization published a completed-value obligation")
	}
	for stmt := range old {
		if _, ok := a.ResolvedOwnedCleanupInitializationOf(stmt); ok {
			t.Fatal("previous snapshot retained a registration")
		}
	}
}
