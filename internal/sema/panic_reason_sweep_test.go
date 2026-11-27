package sema

import (
	"reflect"
	"testing"

	"sec/internal/diagnostics"
)

// TestEveryCanonicalPanicSourceCarriesItsRegistryReason sweeps the panic
// effect producers: each canonical source records exactly its stable
// registry reason (invalid reference generation, contract failure, foreign
// abort, bounds, assertion, checked unreachable, explicit panic, string
// materialization and Arena epoch exhaustion), every one is a panic effect,
// and an open call target records none because no single registered reason
// applies to an unknown callee.
// Rules: rules/errors/panic.md — §§ 13(1)-(2), 13(8), 21.
func TestEveryCanonicalPanicSourceCarriesItsRegistryReason(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Percent int range 0..100

extern "C" fn native(value: int32) int32

fn Reference() Result[void, AllocationError] {
    let mut mem: Arena := Arena {}
    let values := try mem.Alloc[byte](4u)
    let element := ref values[0]
    discard element
    return Ok()
}

fn Contract(value: int) Percent {
    return Percent(value)
}

fn Foreign() int32 {
    return unsafe native(1)
}

fn Bounds(values: int[3], index: int) int {
    return values[index]
}

fn Assertion(value: int) void {
    assert value > 0
}

fn Unreachable() int {
    unreachable
}

fn Explicit() void {
    panic
}

fn Concat(left: string, right: string) string {
    return left + right
}

fn Reset() void {
    let mut buffer: byte[16]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
}

fn Open(callback: fn() void) void {
    callback()
}
`)
	assertSemaErrors(t, errors, nil)
	want := map[string]map[EffectKind][]diagnostics.PanicReasonID{
		"Reference":   {EffectMayPanicInvalidReference: {diagnostics.PanicReasonInvalidReferenceGeneration}},
		"Contract":    {EffectMayPanicContract: {diagnostics.PanicReasonContractFailure}},
		"Foreign":     {EffectMayPanicForeign: {diagnostics.PanicReasonForeignAbort}},
		"Bounds":      {EffectMayPanicBounds: {diagnostics.PanicReasonBoundsFailure}},
		"Assertion":   {EffectMayPanicAssertion: {diagnostics.PanicReasonAssertionFailure}},
		"Unreachable": {EffectMayPanicUnreachable: {diagnostics.PanicReasonCheckedUnreachableReached}},
		"Explicit":    {EffectMayPanicExplicit: {diagnostics.PanicReasonExplicitPanic}},
		"Concat":      {EffectMayPanicStringMaterialization: {diagnostics.PanicReasonStringMaterializationFailure}},
		"Reset":       {EffectMayPanicArenaEpochExhaustion: {diagnostics.PanicReasonArenaEpochExhaustion}},
	}
	graph := analyzer.CallGraph()
	for name, kinds := range want {
		found := map[EffectKind]bool{}
		for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, name)).DirectEffects {
			reasons, expected := kinds[effect.Kind]
			if !expected {
				continue
			}
			found[effect.Kind] = true
			if !reflect.DeepEqual(effect.PanicReasonIDs, reasons) {
				t.Errorf("%s %s reasons = %v, want %v", name, effect.Kind, effect.PanicReasonIDs, reasons)
			}
			if !IsPanicEffectKind(effect.Kind) {
				t.Errorf("%s is not classified as a panic effect", effect.Kind)
			}
			for _, reason := range effect.PanicReasonIDs {
				if _, registered := diagnostics.PanicReasonByID(reason); !registered {
					t.Errorf("%s carries unregistered reason %d", name, reason)
				}
			}
		}
		for kind := range kinds {
			if !found[kind] {
				t.Errorf("%s has no %s effect", name, kind)
			}
		}
	}
	for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, "Open")).DirectEffects {
		if effect.Kind == EffectMayPanicUnknownCallee && len(effect.PanicReasonIDs) != 0 {
			t.Errorf("an open call target cannot name one registered reason: %v", effect.PanicReasonIDs)
		}
	}
}
