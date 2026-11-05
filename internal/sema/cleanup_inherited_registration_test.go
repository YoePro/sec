package sema

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"sec/internal/ast"
)

// MD-125 separates incarnation sites from registrations. Exercise resolved
// source facts, not a simulated runtime: repeated replacement, interleaved
// owners/defers, reinit then replacement, and conditional convergence.
// Rules: rules/memory/destruction.md — §§6.1(4–5),13.2(5–8),13.3(5);
// rules/control-flow/defer.md — §9; rules/compiler/semantic_ir.md — §40(7–10).
func TestInheritedCleanupRegistrationPlans(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_inherited_registration.sec")
	for _, name := range []string{"Repeated", "ReinitThenReplace", "ConditionalThenReplace", "Fallible"} {
		t.Run(name, func(t *testing.T) {
			var statements []*ast.AssignmentStatement
			for stmt, assignment := range a.ownedCleanupAssignments {
				if a.callGraph.nodes[assignment.Executor].Name == name {
					statements = append(statements, stmt)
				}
			}
			sort.Slice(statements, func(i, j int) bool { return tokenBefore(statements[i].Token, statements[j].Token) })
			wantAssignments := 2
			if name == "Fallible" {
				wantAssignments = 1
			}
			if len(statements) != wantAssignments {
				t.Fatalf("assignments = %d", len(statements))
			}
			plan := cleanupPlanFor(t, a, name)
			var kinds []CleanupRegistrationKind
			byAction := map[CleanupActionID]ResolvedCleanupRegistrationSite{}
			for _, registration := range plan.Registrations {
				kinds = append(kinds, registration.Kind)
				if _, duplicate := byAction[registration.Action]; duplicate {
					t.Fatal("duplicate final obligation")
				}
				byAction[registration.Action] = registration
			}
			want := []CleanupRegistrationKind{CleanupRegisterOwned, CleanupRegisterDefer}
			switch name {
			case "Repeated":
				want = []CleanupRegistrationKind{CleanupRegisterOwned, CleanupRegisterDefer, CleanupRegisterOwned, CleanupRegisterDefer}
			case "ReinitThenReplace", "ConditionalThenReplace":
				want = []CleanupRegistrationKind{CleanupRegisterOwned, CleanupRegisterReinitialization, CleanupRegisterDefer}
			}
			if !reflect.DeepEqual(kinds, want) {
				t.Fatalf("registrations = %v, want %v", kinds, want)
			}
			seenIncarnations := map[CleanupActionID]bool{}
			for i, stmt := range statements {
				assignment, _ := a.ResolvedOwnedCleanupAssignmentOf(stmt)
				candidate := assignment.Registration
				if candidate.Action == 0 || seenIncarnations[candidate.Action] {
					t.Fatal("incarnations collapsed")
				}
				seenIncarnations[candidate.Action] = true
				newRegistration := i == 0 && (name == "ReinitThenReplace" || name == "ConditionalThenReplace")
				if newRegistration {
					if assignment.Cleanup.Register != candidate.Action {
						t.Fatal("reinit lost its new registration")
					}
					registration := byAction[candidate.Action]
					conditional := name == "ConditionalThenReplace"
					if registration.IfUnavailable != conditional || registration.Conditional != conditional {
						t.Fatalf("wrong per-path registration: %+v", registration)
					}
					if conditional != (assignment.Cleanup.Inherit == candidate.Binding) {
						t.Fatal("lost available-path inheritance")
					}
				} else {
					if assignment.Cleanup.Inherit != candidate.Binding || assignment.Cleanup.Register != 0 {
						t.Fatalf("replacement relocated the active slot: %+v", assignment.Cleanup)
					}
					if _, registered := byAction[candidate.Action]; registered {
						t.Fatal("replacement added a final obligation")
					}
				}
				initial := a.ownedCleanupInitializations[a.ownedCleanupDeclarations[candidate.Binding]]
				if candidate.Scope != initial.Scope || candidate.Binding != initial.Binding {
					t.Fatal("assignment changed lexical lifetime")
				}
				for _, registration := range plan.Registrations {
					if registration.Defer == nil {
						continue
					}
					if len(registration.Defer.Captures) != 1 {
						t.Fatal("defer lost its capture")
					}
					capture := registration.Defer.Captures[0]
					if capture.Binding != candidate.Binding || capture.Place.RootID != candidate.Place.RootID || capture.RequiredUntil != registration.Action {
						t.Fatalf("defer lost the current destination Place: %+v", capture)
					}
				}
			}
			// Captures denote the same Place as the assignment, never a snapshot
			// or a per-incarnation binding. Non-consuming reads stay valid.
			for _, cancellation := range a.ownedCleanupCancellations {
				for _, fact := range cancellation {
					if fact.Invocation == plan.Invocation && name != "ReinitThenReplace" && name != "ConditionalThenReplace" {
						t.Fatal("replacement canceled the inherited obligation")
					}
				}
			}
			if name == "Fallible" {
				found := false
				for _, exit := range plan.Exits {
					found = found || exit.Kind == CleanupExitErrorPropagation && exit.ProtectedKind == "error"
				}
				if !found {
					t.Fatal("fallible preparation lost its protected failure exit")
				}
			}
		})
	}
}

// Partial roots retain the existing root obligation, including nested masks;
// consumers cannot mutate a later query by changing a detached moved path.
// Rules: rules/memory/destruction.md — §§10,13.2(5),13.3(5);
// rules/compiler/semantic_ir.md — §40(9).
func TestInheritedCleanupPartialFactsDetached(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_transitions.sec")
	for stmt, stored := range a.ownedCleanupAssignments {
		if stored.Previous != CleanupPreviousPartial {
			continue
		}
		plan, _ := a.ResolvedOwnedCleanupAssignmentOf(stmt)
		if plan.Cleanup.Inherit != plan.Registration.Binding || plan.Cleanup.Register != 0 {
			t.Fatal("partial replacement relocated cleanup")
		}
		plan.Moved[0][0] = "changed"
		plan.Cleanup = ResolvedCleanupRegistrationEffect{}
		again, _ := a.ResolvedOwnedCleanupAssignmentOf(stmt)
		if !reflect.DeepEqual(again, stored) {
			t.Fatal("assignment query aliases analyzer state")
		}
		return
	}
	t.Fatal("fixture lost its partial replacement")
}

// A new obligation retains the binding lifetime but starts where reinit executes,
// including repeated loop instances and an outer owner reinitialized in a branch.
// Rules: rules/memory/destruction.md — §§6.1(5),13.1(3),13.3(5);
// rules/compiler/semantic_ir.md — §40(8–9).
func TestInheritedCleanupReinitExecutionScope(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_inherited_registration.sec")
	for _, name := range []string{"LoopReinit", "BranchReinit"} {
		plan := cleanupPlanFor(t, a, name)
		if len(plan.Registrations) != 2 {
			t.Fatalf("%s registrations: %+v", name, plan.Registrations)
		}
		reinit := plan.Registrations[1]
		if reinit.Kind != CleanupRegisterReinitialization || reinit.Scope != plan.Root || reinit.ExecutionScope == plan.Root || reinit.ExecutionScope == 0 || !reinit.Conditional || reinit.IfUnavailable || reinit.PerExecution != (name == "LoopReinit") {
			t.Fatalf("%s reinit lost execution/lifetime distinction: %+v", name, reinit)
		}
	}
}

// Owned by-value parameters are mutable owners just like locals. Replacement
// inherits their entry obligation; after discard, reinit creates a new slot.
// Rules: rules/declarations/functions.md — §6; rules/memory/destruction.md —
// §§6.1(4–5),9.2,13.2(5–8); rules/compiler/semantic_ir.md — §40(7–10).
func TestInheritedCleanupOwnedParameters(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "cleanup_inherited_registration.sec")
	for _, name := range []string{"OwnedParameter", "ReinitParameter"} {
		plan := cleanupPlanFor(t, a, name)
		want := 2
		if name == "ReinitParameter" {
			want = 3
		}
		if len(plan.Registrations) != want || plan.Registrations[0].Kind != CleanupRegisterParameter {
			t.Fatalf("%s: %+v", name, plan.Registrations)
		}
		entry := plan.Registrations[0]
		assignments := 0
		for _, assignment := range a.ownedCleanupAssignments {
			if assignment.Executor != plan.Invocation {
				continue
			}
			assignments++
			candidate := assignment.Registration
			if candidate.Binding != entry.Binding || candidate.Scope != entry.Scope || candidate.Activation != CleanupAfterInitialization {
				t.Fatal("parameter assignment lost its success-only lifetime")
			}
			if assignment.Previous == CleanupPreviousUnavailable {
				if assignment.Cleanup.Register != candidate.Action || assignment.Cleanup.Inherit != 0 {
					t.Fatal("parameter reinit reused entry obligation")
				}
			} else if assignment.Cleanup.Inherit != entry.Binding || assignment.Cleanup.Register != 0 {
				t.Fatal("parameter replacement registered a new slot")
			}
		}
		if assignments != want-1 {
			t.Fatalf("%s assignments: %d", name, assignments)
		}
	}
}

// Preserving the slot does not authorize replacing borrowed storage, nor moving
// or discarding a value a pending defer needs. Rejected sites publish no facts.
// Rules: rules/memory/destruction.md — §13.2(3–4,6); rules/memory/borrowing.md — §21;
// rules/control-flow/defer.md — §§9–11.
func TestInheritedCleanupRejectsInvalidation(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/cleanup_inherited_registration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 4 || len(a.ownedCleanupAssignments) != 0 {
		t.Fatalf("invalid transitions published: %v, assignments: %v", errors, a.ownedCleanupAssignments)
	}
	// The historical lowering boundary remains a valid source program.
	source, err = os.ReadFile("../../testdata/semantic_ir/package17_variants/defer_reads_replaced.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors = analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 0 {
		t.Fatalf("valid replacement/defer rejected: %v", errors)
	}
}
