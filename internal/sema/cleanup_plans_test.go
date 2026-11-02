package sema

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func cleanupPlanFixture(t *testing.T) *Analyzer {
	t.Helper()
	source, err := os.ReadFile("../../testdata/sema/cleanup_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	return analyzer
}

func cleanupPlanFor(t *testing.T, a *Analyzer, name string) ResolvedCleanupPlan {
	t.Helper()
	functions := a.functions[name]
	if len(functions) != 1 {
		t.Fatalf("function %s: %d overloads", name, len(functions))
	}
	plan, ok := a.ResolvedCleanupPlanOf(functions[0])
	if !ok {
		t.Fatalf("no cleanup plan for %s", name)
	}
	return plan
}

func describeCleanupPlan(plan ResolvedCleanupPlan) []string {
	var lines []string
	depth := map[CleanupScopeID]int{}
	for _, scope := range plan.Scopes {
		depth[scope.ID] = depth[scope.Parent] + 1
	}
	for _, registration := range plan.Registrations {
		line := fmt.Sprintf("register %s %d:%d scope-depth=%d", registration.Kind, registration.Source.Line, registration.Source.Column, depth[registration.Scope])
		if registration.PerExecution {
			line += " per-execution"
		}
		if registration.Conditional {
			line += " conditional"
		}
		lines = append(lines, line)
	}
	for _, exit := range plan.Exits {
		line := fmt.Sprintf("exit %s %d:%d leaves=%d", exit.Kind, exit.Source.Line, exit.Source.Column, len(exit.Exited))
		if exit.FunctionExit {
			line += " function"
		}
		if !exit.RunsCleanup {
			line += " no-cleanup"
		}
		if exit.ProtectedKind != "" {
			line += " protects=" + exit.ProtectedKind
		}
		lines = append(lines, line)
	}
	return lines
}

// P17-21 … P17-24: each invocation's detached cleanup plan lists its
// registration sites in source order — with per-execution loop multiplicity
// and conditional sites marked — and its exit edges: block fallthrough,
// break/continue up to the loop, returns and error propagation as function
// exits protecting the leaving value, and panic without cleanup.
func TestResolvedCleanupPlans(t *testing.T) {
	a := cleanupPlanFixture(t)
	want := map[string][]string{
		"Order": {
			"register owned-initialization 25:9 scope-depth=1",
			"register defer 26:5 scope-depth=1",
			"register owned-initialization 29:9 scope-depth=1",
			"register owned-initialization 31:13 scope-depth=2 conditional",
			"register owned-initialization 36:13 scope-depth=2 per-execution conditional",
			"register defer 37:9 scope-depth=2 per-execution conditional",
			"exit return 33:9 leaves=2 function protects=return-value",
			"exit fallthrough 35:20 leaves=1",
			"exit continue 41:13 leaves=2",
			"exit break 44:13 leaves=2",
			"exit return 48:5 leaves=1 function protects=return-value",
		},
		"Propagate": {
			"register owned-initialization 52:9 scope-depth=1",
			"exit error-propagation 52:18 leaves=1 function protects=error",
			"exit return 54:5 leaves=1 function protects=return-value",
		},
		"Crash": {
			"register owned-initialization 58:9 scope-depth=1",
			"exit panic 60:5 leaves=0 no-cleanup",
		},
		"Tail": {
			"register owned-initialization 64:9 scope-depth=1",
			"exit fallthrough 63:16 leaves=1 function",
		},
	}
	for name, expected := range want {
		got := describeCleanupPlan(cleanupPlanFor(t, a, name))
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s cleanup plan:\n%s\nwant:\n%s", name, strings.Join(got, "\n"), strings.Join(expected, "\n"))
		}
	}
}

// The plan is detached and stable: repeated queries are equal and mutating
// one result does not change the next.
func TestResolvedCleanupPlansDetached(t *testing.T) {
	a := cleanupPlanFixture(t)
	first := cleanupPlanFor(t, a, "Order")
	first.Exits[0].Exited[0] = 999
	first.Registrations[0].Place.Projections = append(first.Registrations[0].Place.Projections, PlaceProjection{})
	second := cleanupPlanFor(t, a, "Order")
	if second.Exits[0].Exited[0] == 999 || len(second.Registrations[0].Place.Projections) != 0 {
		t.Fatal("cleanup plan query is not detached")
	}
	if !reflect.DeepEqual(describeCleanupPlan(second), describeCleanupPlan(cleanupPlanFor(t, a, "Order"))) {
		t.Fatal("cleanup plan queries differ")
	}
}

// A moved payload binding is registered in a scope of its own that ends with the
// arm; a borrowed or trivial binding registers nothing.
// Rules: rules/control-flow/flowcontrol_match.md — §§ 12–13;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 63, 111.
func TestResolvedCleanupPlansOwnedMatchBindings(t *testing.T) {
	a, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

fn Fresh() Option[Handle] {
    return None
}

fn Moved() void {
    match Fresh() {
        Some(handle) => {
        }
        None => {
        }
    }
}

fn Returned() int {
    match Fresh() {
        Some(handle) => {
            return handle.id
        }
        None => {
            return 0
        }
    }
}

fn Borrowed() int {
    let option := Fresh()
    match option {
        Some(ref handle) => {
            return handle.id
        }
        None => {
            return 0
        }
    }
}

fn Present() void {
    if Fresh() is Some(handle) {
    }
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	bindings := func(name string) (count int, exits []CleanupExitKind) {
		plan := cleanupPlanFor(t, a, name)
		scopes := map[CleanupScopeID]bool{}
		for _, registration := range plan.Registrations {
			if registration.Kind == CleanupRegisterMatchBinding {
				count++
				scopes[registration.Scope] = true
				if registration.Scope == plan.Root || !registration.Conditional {
					t.Errorf("%s: binding registered outside its arm: %+v", name, registration)
				}
			}
		}
		for _, exit := range plan.Exits {
			for _, scope := range exit.Exited {
				if scopes[scope] {
					exits = append(exits, exit.Kind)
					break
				}
			}
		}
		return count, exits
	}
	for name, want := range map[string]struct {
		count int
		exits []CleanupExitKind
	}{
		"Moved":    {1, []CleanupExitKind{CleanupExitFallthrough}},
		"Returned": {1, []CleanupExitKind{CleanupExitReturn}},
		"Borrowed": {0, nil},
		"Present":  {1, []CleanupExitKind{CleanupExitFallthrough}},
	} {
		count, exits := bindings(name)
		if count != want.count || !reflect.DeepEqual(exits, want.exits) {
			t.Errorf("%s: %d bindings, exits %v; want %d, %v", name, count, exits, want.count, want.exits)
		}
	}
}

// A try binding consumes a named owner on every path, so a later use is a
// compile error and a later assignment is a reinitialization.
// Rules: rules/errors/errorhandling.md — § 29.
func TestTryConsumesNamedOwner(t *testing.T) {
	const declarations = `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

fn Fresh() Result[Handle, IndexError] {
    return Ok(Handle { id: 1 })
}

fn Plain() Result[int, IndexError] {
    return Ok(1)
}

fn Look(result: ref Result[Handle, IndexError]) void {
}
`
	_, errors := analyzeSourceWithAnalyzerRaw(t, declarations+`
fn T() Result[int, IndexError] {
    let result := Fresh()
    let handle := try result
    Look(ref result)
    return Ok(handle.id)
}
`)
	if len(errors) == 0 {
		t.Fatal("use after try of a named owner was accepted")
	}
	_, errors = analyzeSourceWithAnalyzerRaw(t, declarations+`
fn T() Result[int, IndexError] {
    let mut result := Fresh()
    let handle := try result
    result = Fresh()
    let plain := Plain()
    let first := try plain
    let second := try plain
    return Ok(handle.id + first + second)
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
}

// A union whose payload moved on one arm is conditional as a whole: assignment
// and discard are legal, and discard leaves it unavailable on every path.
// Rules: rules/memory/destruction.md — §§ 12.3, 13.3, 35.5.
func TestConditionalPayloadAssignmentAndDiscard(t *testing.T) {
	const declarations = `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

fn Fresh() Option[Handle] {
    return None
}

fn Sink(->handle: Handle) void {
}

fn Look(option: ref Option[Handle]) void {
}
`
	const moved = `
    let mut option := Fresh()
    match option {
        Some(handle) => {
            Sink(<-handle)
        }
        None => {
        }
    }
`
	for name, test := range map[string]struct {
		body    string
		invalid bool
	}{
		"assign":             {body: "    option = Fresh()\n"},
		"discard":            {body: "    discard option\n"},
		"discard twice":      {body: "    discard option\n    discard option\n"},
		"discard and assign": {body: "    discard option\n    option = Fresh()\n    Look(ref option)\n"},
		"use":                {body: "    Look(ref option)\n", invalid: true},
		"use after discard":  {body: "    discard option\n    Look(ref option)\n", invalid: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzerRaw(t, declarations+"\nfn T() void {"+moved+test.body+"}\n")
			if test.invalid != (len(errors) != 0) {
				t.Fatalf("invalid=%v errors=%v", test.invalid, errors)
			}
		})
	}
}
