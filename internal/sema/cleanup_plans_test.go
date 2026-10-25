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
