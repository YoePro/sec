package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A use of Arena storage after the Reset that ended its epoch is S1222 and
// names the rejected use, the allocation origin with its epoch, the
// invalidating Reset and safe restructurings.
// Rules: rules/memory/arena.md — §§ 38(1–4), 44(1);
// rules/memory/lifetime_analysis.md — mentor diagnostics.
func TestArenaUseAfterInvalidationMentor(t *testing.T) {
	source := `module main

fn Scratch() Result[int, AllocationError] {
    let mut arena := try Arena.WithCapacity(64u)
    let slot := try arena.New[int]()
    *slot = 4
    arena.Reset()
    return Ok(*slot)
}
`
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 1 {
		t.Fatalf("errors = %v", errors)
	}
	err := errors[0]
	if err.ID != diagnostics.ArenaUseAfterInvalidation || err.Line != 8 || err.PreviousLine != 5 {
		t.Fatalf("stale use = %+v", err)
	}
	if !strings.Contains(err.RelatedLabel, "allocated from arena arena in epoch 0") {
		t.Errorf("origin label = %q", err.RelatedLabel)
	}
	if len(err.Related) != 1 || err.Related[0].Line != 7 || !strings.Contains(err.Related[0].Label, "Reset of arena arena ends that epoch") {
		t.Errorf("invalidating operation = %+v", err.Related)
	}
	for _, want := range []string{"before that operation", "allocate a fresh value", "owned value"} {
		if !strings.Contains(err.Help, want) {
			t.Errorf("help lacks %q: %s", want, err.Help)
		}
	}
}

// Destruction mentor diagnostics name the affected value, where its
// ownership responsibility was established, the earlier move, discard or
// borrow that changed it, and a safe source fix, for definite, conditional
// and replacement cases.
// Rules: rules/memory/destruction.md — §§ 9–12, 33; rules/tooling/diagnostics.md — §§ 7, 12.
func TestDestructionMentorDiagnostics(t *testing.T) {
	source := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

fn Consume(-> r: Res) int {
    return r.id
}

fn Moved() int {
    let r := Res { id: 1 }
    let n := Consume(<-r)
    return n + r.id
}

fn Discarded() int {
    let r := Res { id: 1 }
    discard r
    return r.id
}

fn Conditional(flag: bool) int {
    let r := Res { id: 1 }
    if flag {
        discard Consume(<-r)
    }
    return r.id
}

fn Replaced() int {
    let mut r := Res { id: 1 }
    let view := ref r
    r = Res { id: 2 }
    return view.id
}
`
	errors := analyzeSourceRaw(t, source)
	want := []struct {
		id      string
		line    int
		related []int
		help    string
	}{
		{diagnostics.UseAfterMove, 19, []int{18, 17}, "use r before that operation"},
		{diagnostics.UseAfterDiscard, 25, []int{24, 23}, "keep the value until its last use"},
		{diagnostics.ConditionallyUnavailableUse, 33, []int{31, 29}, "restructure the control flow"},
		{diagnostics.BorrowMutationConflict, 39, []int{38}, "end the conflicting borrow"},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, expected := range want {
		err := errors[index]
		if err.ID != expected.id || err.Line != expected.line || !strings.Contains(err.Message, "r ") {
			t.Errorf("diagnostic %d = %s %d %q, want %s at %d", index, err.ID, err.Line, err.Message, expected.id, expected.line)
			continue
		}
		lines := map[int]bool{}
		for _, location := range err.RelatedLocations() {
			lines[location.Line] = true
		}
		for _, line := range expected.related {
			if !lines[line] {
				t.Errorf("%s: no related location at line %d: %+v", err.ID, line, err.RelatedLocations())
			}
		}
		if !strings.Contains(err.Help, expected.help) {
			t.Errorf("%s: help lacks %q: %s", err.ID, expected.help, err.Help)
		}
	}
}
