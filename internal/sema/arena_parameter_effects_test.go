package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const arenaParameterEffectHelpers = `module main

fn Fill(arena: ref mut Arena) void {
    arena.Reset()
}

fn Forward(arena: ref mut Arena) void {
    Fill(arena)
}

fn Allocate(arena: ref mut Arena) Result[ref mut byte[], AllocationError] {
    let values := try arena.Alloc[byte](4u)
    return Ok(values)
}

fn Ping(arena: ref mut Arena, depth: int) void {
    if depth > 0 {
        Pong(arena, depth - 1)
    }
}

fn Pong(arena: ref mut Arena, depth: int) void {
    if depth == 0 {
        arena.Reset()
        return
    }
    Ping(arena, depth - 1)
}

`

// A Reset through a ref mut Arena parameter is part of the callee's summary:
// calls that may Reset the caller's Arena advance its epoch, directly,
// through forwarding helpers, through mutual recursion and through a local
// alias, and fail the Reset dependency check while a deferred use is live
// (arena.md §§36, 44, 68).
func TestCalleeArenaResetInvalidatesCallerAllocations(t *testing.T) {
	for name, body := range map[string]string{
		"direct":    "    Fill(mem)",
		"forwarded": "    Forward(mem)",
		"recursive": "    Ping(mem, 3)",
		"alias":     "    let alias := ref mut mem\n    Fill(alias)",
	} {
		source := arenaParameterEffectHelpers + "fn Use() Result[void, AllocationError] {\n    let mut mem: Arena := Arena {}\n    let values := try mem.Alloc[byte](16u)\n" + body + "\n    discard values\n    return Ok()\n}\n"
		errors := analyzeSourceRaw(t, source)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot use values after arena mem was reset") {
			t.Errorf("%s: errors = %v", name, errors)
		}
	}
	deferred := arenaParameterEffectHelpers + "fn Use() Result[void, AllocationError] {\n    let mut mem: Arena := Arena {}\n    let values := try mem.Alloc[byte](16u)\n    defer { discard values[0] }\n    Fill(mem)\n    return Ok()\n}\n"
	errors := analyzeSourceRaw(t, deferred)
	if len(errors) != 1 || errors[0].ID != diagnostics.ArenaResetLiveDependency || !strings.Contains(errors[0].Message, "cannot pass arena mem to Fill, which may Reset it,") {
		t.Fatalf("deferred errors = %v", errors)
	}
}

// Calls that never Reset the passed Arena keep earlier allocations valid, and
// summaries record exactly the resetting Arena parameters.
func TestCalleeArenaParameterSummaries(t *testing.T) {
	source := arenaParameterEffectHelpers + "fn Use() Result[void, AllocationError] {\n    let mut mem: Arena := Arena {}\n    let values := try mem.Alloc[byte](16u)\n    let more := try Allocate(mem)\n    discard values\n    discard more\n    return Ok()\n}\n"
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	assertSemaErrors(t, errors, nil)
	for name, want := range map[string][]int{"Fill": {0}, "Forward": {0}, "Ping": {0}, "Pong": {0}, "Allocate": nil, "Use": nil} {
		functions := analyzer.functions[name]
		if len(functions) != 1 || !reflect.DeepEqual(functions[0].ArenaParameterResets, want) {
			t.Errorf("%s resets = %v, want %v", name, functions, want)
		}
	}
}
