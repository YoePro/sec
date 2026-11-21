package sema

import (
	"os"
	"strings"
	"testing"
)

// TestLoopArenaGenerationReplay verifies stale reads on later body/condition
// evaluations, including continue-only backedges. Break paths cannot create a
// second iteration; freshly allocated per-iteration storage remains valid.
// Rules: rules/control-flow/flowcontrol_while.md §§14,23,28–29;
// rules/control-flow/flowcontrol_for.md §§30–31,42;
// rules/memory/arena.md §38.
func TestLoopArenaGenerationReplay(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/loop_arena_generation/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := analyzeSourceRaw(t, string(source))
		if name == "valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			continue
		}
		if len(errors) != 5 {
			t.Fatalf("errors = %+v", errors)
		}
		for _, err := range errors {
			if !strings.Contains(err.Message, "cannot use storage after arena mem was reset") {
				t.Errorf("unexpected error: %+v", err)
			}
		}
	}
}

// TestLoopArenaGenerationEdges keeps continue/fallthrough joins detached from
// their snapshots and excludes breaks from a later iteration. Equal generations
// on different ArenaDomain identities are still different header states.
// Rules: rules/control-flow/flowcontrol_while.md §§14,23,28–29;
// rules/control-flow/flowcontrol_for.md §§30–31,42; rules/memory/arena.md §§4.5,38.
func TestLoopArenaGenerationEdges(t *testing.T) {
	entry := map[string]int{"arena": 0}
	body := map[string]int{"arena": 2}
	continued := map[string]int{"arena": 3}
	frame := loopBreakFrame{
		continues: []loopEdgeState{{arenaGenerations: continued}},
		breaks:    []loopEdgeState{{arenaGenerations: map[string]int{"arena": 99}}},
	}
	for _, fallsThrough := range []bool{false, true} {
		header := loopBackedgeArenaGenerationState(entry, body, frame, fallsThrough)
		if header["arena"] != 3 {
			t.Fatalf("wrong next-iteration generation: %v", header)
		}
		header["arena"] = 4
		if entry["arena"] != 0 || body["arena"] != 2 || continued["arena"] != 3 {
			t.Fatal("join mutated an edge snapshot")
		}
	}
	if header := loopBackedgeArenaGenerationState(entry, body, loopBreakFrame{breaks: frame.breaks}, false); header["arena"] != 0 {
		t.Fatal("break created a backedge", header)
	}
	if arenaGenerationStatesEqual(entry, body) {
		t.Fatal("generation-only change did not require replay")
	}
	if arenaGenerationStatesEqual(map[string]int{"first": 0}, map[string]int{"second": 0}) {
		t.Fatal("different ArenaDomains compared equal")
	}
	if !arenaGenerationStatesEqual(entry, copyArenaGenerations(entry)) {
		t.Fatal("identical generation snapshot compared unequal")
	}
}
