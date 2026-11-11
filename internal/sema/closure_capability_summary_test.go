package sema

import "testing"

// Closure analysis publishes the callable capability inferred from the
// environment use (shared fn, mut fn after mutating an owned capture, -> fn
// after moving state out) and whether the environment is move-only, the same
// facts as the lambda's function type.
//
// Rules:
//   - rules/analysis/closure_analysis.md — § 3(2), § 9.3(3), § 81(1)
//   - rules/declarations/lambda-functions.md — §§ 27–28
func TestClosureCreationSummariesCarryCallableCapability(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

fn Shared() int {
    let factor := 2
    let multiply := capture(factor) fn(value: int) int {
        return value * factor
    }
    return multiply(3)
}

fn Mutable() int {
    let count := 0
    let mut next := capture(count) fn() int {
        count += 1
        return count
    }
    return next()
}

fn Consuming() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let owned := take()
    return owned.id
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	creations := analyzer.ClosureCreations()
	want := []struct {
		capability CallableCapability
		moveOnly   bool
	}{
		{CallableShared, false},
		{CallableMutable, false},
		{CallableConsuming, true},
	}
	if len(creations) != len(want) {
		t.Fatalf("closure creations = %d, want %d", len(creations), len(want))
	}
	for i, creation := range creations {
		if creation.Capability != want[i].capability || creation.EnvironmentMoveOnly != want[i].moveOnly {
			t.Errorf("creation %d at %d: capability %s move-only %v, want %s %v", i, creation.Source.Line, creation.Capability, creation.EnvironmentMoveOnly, want[i].capability, want[i].moveOnly)
		}
	}
}
