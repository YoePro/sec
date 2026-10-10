package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const specializationEffectsSource = `module main

interface Named {
    fn Name() int
}

type Loud struct {
    level: int,
}

impl Loud implements Named {
    fn Name() int {
        let text := try $"loud {self.level}" {
            Err(_) => ""
        }
        discard text
        panic "loud"
    }
}

type Quiet struct {
    level: int,
}

impl Quiet implements Named {
    fn Name() int {
        return self.level
    }
}

fn GenericCall[T: Named](value: ref T) int {
    return value.Name()
}

fn Forward[U: Named](value: ref U) int {
    return GenericCall(value)
}

@noAlloc
fn AllocatesThroughSpecialization(value: ref Loud) int {
    return GenericCall[Loud](value)
}

@noPanic
fn PanicsThroughForwardedSpecialization(value: ref Loud) int {
    return Forward(value)
}

@noAlloc
@noPanic
fn QuietSpecializationIsClean(value: ref Quiet) int {
    return Forward[Quiet](value)
}
`

// Concrete generic specializations are executable call-graph nodes: they
// carry the template's effects and replace constraint calls with static edges
// to the concrete methods, also through templates that forward their
// parameters, so @noAlloc and @noPanic see what a specialization executes.
//
// Rules:
//   - rules/compiler/monomorphization.md — §§12.4–12.5
//   - rules/analysis/call_graph.md — "Callable node"
//   - rules/memory/allocation.md — § 24; rules/errors/panic.md — § 21
func TestGenericSpecializationsCarryConcreteEffects(t *testing.T) {
	errors := analyzeSourceRaw(t, specializationEffectsSource)
	found := map[string]bool{}
	for _, err := range errors {
		switch {
		case err.ID == diagnostics.NoAllocViolation && strings.Contains(err.Message, "AllocatesThroughSpecialization") && strings.Contains(err.Message, "GenericCall -> Loud.Name"):
			found["alloc"] = true
		case err.ID == diagnostics.NoPanicViolation && strings.Contains(err.Message, "PanicsThroughForwardedSpecialization") && strings.Contains(err.Message, "Forward -> GenericCall -> Loud.Name"):
			found["panic"] = true
		default:
			t.Errorf("unexpected error: %+v", err)
		}
	}
	if !found["alloc"] || !found["panic"] {
		t.Fatalf("errors = %v", errors)
	}
}
