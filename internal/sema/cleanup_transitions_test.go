package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
)

// Source producers retain exact canonical ownership ends and distinguish
// replacement from reinitialization after RHS effects, including CFG joins.
// Rules: rules/memory/copy_move.md — §§12.1–12.3;
// rules/memory/destruction.md — §6.1(2–3), §§9–13.
func TestOwnedCleanupTransitions(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_transitions.sec")
	seen := map[string]CleanupPreviousValue{}
	for stmt := range a.ownedCleanupAssignments {
		plan, ok := a.ResolvedOwnedCleanupAssignmentOf(stmt)
		if !ok || plan.Registration.Activation != CleanupAfterInitialization {
			t.Fatal("assignment lost success-only activation")
		}
		registration := plan.Registration
		decl := a.ownedCleanupDeclarations[registration.Binding]
		initial := a.ownedCleanupInitializations[decl]
		if registration.Action == initial.Action || registration.Binding != initial.Binding || registration.Scope != initial.Scope || registration.Invocation != initial.Invocation {
			t.Fatal("assignment reused the old action or changed the binding's lifetime")
		}
		seen[a.callGraph.nodes[plan.Executor].Name] = plan.Previous
	}
	want := map[string]CleanupPreviousValue{
		"MoveAndReinit": CleanupPreviousUnavailable, "Replace": CleanupPreviousAvailable,
		"DiscardAndReinit": CleanupPreviousUnavailable, "Conditional": CleanupPreviousConditional,
		"Partial": CleanupPreviousPartial, "Fallible": CleanupPreviousAvailable,
		"ConsumeAndReplace": CleanupPreviousUnavailable,
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("assignment states = %v, want %v", seen, want)
	}
	cancellations := map[string][]ResolvedOwnedCleanupCancellation{}
	for site := range a.ownedCleanupCancellations {
		plans := a.ResolvedOwnedCleanupCancellationsAt(site.File, site.Line, site.Column)
		for _, plan := range plans {
			if plan.Binding == 0 || plan.Place.RootID == 0 || plan.Reason == "" {
				t.Fatal("cancellation lost canonical ownership target")
			}
			name := a.callGraph.nodes[plan.Invocation].Name
			cancellations[name] = append(cancellations[name], plan)
		}
	}
	for name, count := range map[string]int{
		"MoveAndReinit": 1, "DiscardAndReinit": 1, "Conditional": 1, "Partial": 1,
		"PartialDiscard": 1, "Give": 1, "ConsumeAndReplace": 1, "Pass": 1,
		"ConditionalDiscard": 2, "RedundantDiscard": 1,
	} {
		if len(cancellations[name]) != count {
			t.Errorf("%s cancellations = %v, want %d", name, cancellations[name], count)
		}
	}
	for _, name := range []string{"Partial", "PartialDiscard"} {
		if plans := cancellations[name]; len(plans) == 1 && plans[0].Place.String() != "pair.first" {
			t.Fatal("partial cancellation ended the whole owner")
		}
	}
}

// Queries detach nested facts and never infer or allocate. Reanalysis on new
// source nodes drops the old plans while reproducing semantic site identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§73,79–80.
func TestOwnedCleanupTransitionQueries(t *testing.T) {
	a, program := analyzeDeferPlanFixture(t, "cleanup_transitions.sec")
	reference, _ := analyzeDeferPlanFixture(t, "cleanup_transitions.sec")
	wantAssignments := map[CleanupActionID]ResolvedOwnedCleanupAssignment{}
	for _, plan := range reference.ownedCleanupAssignments {
		wantAssignments[plan.Registration.Action] = plan
	}
	beforeTypes, beforeActions := len(a.expressionTypes), len(a.cleanupIdentities.actions)
	for stmt, stored := range a.ownedCleanupAssignments {
		plan, _ := a.ResolvedOwnedCleanupAssignmentOf(stmt)
		plan.Registration.Place.Root = "changed"
		for id := range plan.Registration.Destruction.Plans {
			delete(plan.Registration.Destruction.Plans, id)
		}
		again, _ := a.ResolvedOwnedCleanupAssignmentOf(stmt)
		if !reflect.DeepEqual(again, wantAssignments[stored.Registration.Action]) {
			t.Fatal("assignment query aliases analyzer-owned data")
		}
	}
	for site := range a.ownedCleanupCancellations {
		plans := a.ResolvedOwnedCleanupCancellationsAt(site.File, site.Line, site.Column)
		for index := range plans {
			plans[index].Place.Root = "changed"
			if len(plans[index].Place.Projections) != 0 {
				plans[index].Place.Projections[0].Name = "changed"
			}
		}
		again := a.ResolvedOwnedCleanupCancellationsAt(site.File, site.Line, site.Column)
		want := reference.ResolvedOwnedCleanupCancellationsAt(site.File, site.Line, site.Column)
		if !reflect.DeepEqual(again, want) {
			t.Fatal("cancellation query aliases analyzer-owned data")
		}
	}
	if _, ok := a.ResolvedOwnedCleanupAssignmentOf(&ast.AssignmentStatement{}); ok || a.ResolvedOwnedCleanupCancellationsAt("unknown.sec", 1, 1) != nil {
		t.Fatal("unknown use acquired facts")
	}
	if beforeTypes != len(a.expressionTypes) || beforeActions != len(a.cleanupIdentities.actions) {
		t.Fatal("query allocated or inferred")
	}
	oldAssignments, oldCancellations := a.ownedCleanupAssignments, a.ownedCleanupCancellations
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(oldAssignments, a.ownedCleanupAssignments) || !reflect.DeepEqual(oldCancellations, a.ownedCleanupCancellations) {
		t.Fatal("reanalysis changed transition facts")
	}
}

// Rejected replacement/self-move and borrowed-source move publish no accepted
// transition at the invalid source sites.
// Rules: rules/memory/copy_move.md — §12.1(2), §13;
// rules/memory/borrowing.md — §21.
func TestOwnedCleanupRejectedTransitions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/cleanup_transitions_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 3 || len(a.ownedCleanupAssignments) != 0 || len(a.ownedCleanupCancellations) != 0 {
		t.Fatalf("rejected transitions published plans: errors=%v assignments=%v cancellations=%v", errors, a.ownedCleanupAssignments, a.ownedCleanupCancellations)
	}
}
