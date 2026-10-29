package semantic

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestNestedSwitchLoopControlTargets verifies actual CFG successors, not only
// printed branch names: switches introduce no loop targets; nested loops do,
// and leaving an inner loop restores the surrounding loop's authority.
// The same fixture executes through the sole native compilation track.
// Rules: rules/control-flow/flowcontrol_switch.md — §§16,31;
// rules/control-flow/flowcontrol_while.md — §§13–17;
// rules/control-flow/flowcontrol_for.md — §§30–31;
// rules/compiler/semantic_ir.md — §§66–68.
func TestNestedSwitchLoopControlTargets(t *testing.T) {
	const fixture = "../../../testdata/build/native/switch_loop_control.sec"
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// Source annotations identify the expected enclosing loop independently
	// of the builder's loop ID and destination choice. Loop numbers follow
	// source nesting in each function (outer first), not switch nesting.
	targetAtLine := map[int]int{}
	for index, line := range strings.Split(string(data), "\n") {
		if _, marker, found := strings.Cut(line, "// loop-target: "); found {
			number, err := strconv.Atoi(marker)
			if err != nil || number < 1 {
				t.Fatalf("invalid fixture annotation on line %d", index+1)
			}
			targetAtLine[index+1] = number
		}
	}
	module := buildForFixture(t, fixture)
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	wantLoops := map[string]int{"WhileSwitch": 1, "ForSwitch": 1, "NestedLoops": 2, "FallthroughLoop": 1, "DefaultSwitch": 1}
	seenLines := map[int]bool{}
	for name, count := range wantLoops {
		function := loopFunction(t, module, name)
		if len(function.Loops) != count {
			t.Fatalf("%s: switches changed loop record count: %d", name, len(function.Loops))
		}
		for _, block := range function.Blocks {
			for _, op := range block.Operations {
				if op.LoopEdge != LoopEdgeBreak && op.LoopEdge != LoopEdgeContinue {
					continue
				}
				index, annotated := targetAtLine[op.Location.Line]
				if !annotated || index > len(function.Loops) || seenLines[op.Location.Line] {
					t.Fatalf("%s: missing/duplicate loop-control source: %+v", name, op)
				}
				seenLines[op.Location.Line] = true
				loop := function.Loops[index-1]
				target := loop.ConditionBlock
				if op.LoopEdge == LoopEdgeBreak {
					target = loop.ExitBlock
				}
				if op.Kind != OpBranch || op.LoopID != loop.ID || target == 0 || len(op.Successors) != 1 || op.Successors[0].Block != target {
					t.Fatalf("%s line %d: %s must target loop %d block %d, got %+v", name, op.Location.Line, op.LoopEdge, loop.ID, target, op)
				}
			}
		}
	}
	if len(seenLines) != len(targetAtLine) || len(seenLines) != 12 {
		t.Fatalf("covered %d of %d source loop-control statements, want 12", len(seenLines), len(targetAtLine))
	}
}
