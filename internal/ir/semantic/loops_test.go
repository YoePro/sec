package semantic

import (
	"errors"
	"strings"
	"testing"
)

const whileLoopSource = `module main

enum Step {
    Go
    Stop
}

fn Count(limit: int) int {
    let mut i := 0
    let mut total := 0
    while i < limit {
        if i == 3 {
            i = i + 1
            continue
        }
        if total > 100 {
            break
        }
        total = total + i
        i = i + 1
    }
    return total
}

fn Spin(stop: bool) int {
    while true {
        if stop {
            break
        }
    }
    return 0
}

fn Forever() int {
    while true {
    }
}

fn Never() int {
    while false {
    }
    return 1
}

fn Grid(n: int) int {
    let mut row := 0
    let mut hits := 0
    while row < n {
        let mut col := 0
        while col < n {
            if col > row {
                break
            }
            hits = hits + 1
            col = col + 1
        }
        row = row + 1
    }
    return hits
}

fn Drive(step: Step) int {
    let mut count := 0
    while count < 10 {
        match step {
            Step.Go => {
                count = count + 1
            }
            Step.Stop => {
                break
            }
        }
    }
    return count
}
`

func loopFunction(t *testing.T, module *Module, name string) *Function {
	t.Helper()
	for _, function := range module.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("function %s is missing", name)
	return nil
}

func loopEdges(function *Function) map[LoopID]map[LoopEdge]int {
	edges := map[LoopID]map[LoopEdge]int{}
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.LoopEdge == "" {
				continue
			}
			if edges[op.LoopID] == nil {
				edges[op.LoopID] = map[LoopEdge]int{}
			}
			edges[op.LoopID][op.LoopEdge]++
		}
	}
	return edges
}

// A while loop becomes explicit loop CFG: one enter edge into the condition
// block, one condition edge selecting body or exit, back and continue edges to
// the condition, and break edges to the exit of the innermost loop. Sema's
// while-flow decision selects the constant forms, and loop-carried state is
// reloaded from mutable storage on every iteration.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 66 "Loop CFG", § 67(4)
//   - rules/control-flow/flowcontrol_while.md — §§ 4, 13–15, 17, 19–20, 29
func TestWhileLoopBuildsExplicitLoopCFG(t *testing.T) {
	module, err := analyzedModule(t, whileLoopSource, 13)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}

	count := loopFunction(t, module, "Count")
	if len(count.Loops) != 1 {
		t.Fatalf("Count loops = %+v", count.Loops)
	}
	loop := count.Loops[0]
	if loop.Kind != LoopWhile || loop.ConditionKnown || !loop.ContinuesAfter || loop.BodyBlock == 0 || loop.ExitBlock == 0 {
		t.Fatalf("Count loop record = %+v", loop)
	}
	edges := loopEdges(count)[loop.ID]
	want := map[LoopEdge]int{LoopEdgeEnter: 1, LoopEdgeCondition: 1, LoopEdgeBack: 1, LoopEdgeContinue: 1, LoopEdgeBreak: 1}
	for edge, n := range want {
		if edges[edge] != n {
			t.Errorf("Count %s edges = %d, want %d", edge, edges[edge], n)
		}
	}
	condition := count.Blocks[loop.ConditionBlock]
	if condition.Operations[0].Kind != OpStorageLoad {
		t.Errorf("condition does not reload loop-carried state: %+v", condition.Operations[0])
	}

	spin := loopFunction(t, module, "Spin").Loops[0]
	if !spin.ConditionKnown || !spin.ConditionValue || !spin.ContinuesAfter || spin.ExitBlock == 0 {
		t.Errorf("Spin loop record = %+v", spin)
	}
	forever := loopFunction(t, module, "Forever")
	if record := forever.Loops[0]; !record.ConditionKnown || record.ContinuesAfter || record.ExitBlock != 0 {
		t.Errorf("Forever loop record = %+v", record)
	}
	if countOperations(forever, OpReturn) != 0 {
		t.Errorf("non-continuing loop reached a return")
	}
	never := loopFunction(t, module, "Never").Loops[0]
	if !never.ConditionKnown || never.ConditionValue || never.BodyBlock != 0 || !never.ContinuesAfter {
		t.Errorf("Never loop record = %+v", never)
	}

	grid := loopFunction(t, module, "Grid")
	if len(grid.Loops) != 2 {
		t.Fatalf("Grid loops = %+v", grid.Loops)
	}
	gridEdges := loopEdges(grid)
	if gridEdges[1][LoopEdgeBreak] != 0 || gridEdges[2][LoopEdgeBreak] != 1 {
		t.Errorf("break did not target the innermost loop: %v", gridEdges)
	}
	if edges := loopEdges(loopFunction(t, module, "Drive"))[1]; edges[LoopEdgeBreak] != 1 || edges[LoopEdgeBack] != 1 {
		t.Errorf("match arm break edges = %v", edges)
	}

	text := Format(module)
	for _, fragment := range []string{"loops {", "[loop=@1 edge=continue]", "[loop=@2 edge=break]", "exit=none constant=true continues=false", "body=none exit="} {
		if !strings.Contains(text, fragment) {
			t.Errorf("format is missing %q", fragment)
		}
	}
	if again, err := analyzedModule(t, whileLoopSource, 13); err != nil || Format(again) != text {
		t.Errorf("loop IR is not deterministic: %v", err)
	}
}

// Packages before 13 keep rejecting while loops at an explicit boundary.
func TestWhileLoopIsRejectedBeforePackage13(t *testing.T) {
	_, err := analyzedModule(t, whileLoopSource, 12)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != "while loop" {
		t.Fatalf("err = %v, want unsupported while loop", err)
	}
}

// The verifier ties every loop record to the edges that carry its identity.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 66(1) "Loop CFG"
func TestVerifierRejectsMalformedLoops(t *testing.T) {
	findEdge := func(function *Function, edge LoopEdge) *Operation {
		for _, block := range function.Blocks {
			for index := range block.Operations {
				if block.Operations[index].LoopEdge == edge {
					return &block.Operations[index]
				}
			}
		}
		t.Fatalf("no %s edge", edge)
		return nil
	}
	tests := []struct {
		name   string
		mutate func(*Function)
		want   string
	}{
		{"break to the condition", func(f *Function) {
			findEdge(f, LoopEdgeBreak).Successors[0].Block = f.Loops[0].ConditionBlock
		}, "break edge targets"},
		{"continue to the exit", func(f *Function) {
			findEdge(f, LoopEdgeContinue).Successors[0].Block = f.Loops[0].ExitBlock
		}, "continue edge targets"},
		{"unmarked entry", func(f *Function) {
			op := findEdge(f, LoopEdgeEnter)
			op.LoopID, op.LoopEdge = 0, ""
		}, "outside its loop edges"},
		{"runtime condition marked non-continuing", func(f *Function) {
			f.Loops[0].ContinuesAfter = false
		}, "marked non-continuing"},
		{"edge without loop", func(f *Function) {
			findEdge(f, LoopEdgeBack).LoopID = 0
		}, "has no loop"},
		{"unknown loop", func(f *Function) {
			findEdge(f, LoopEdgeBack).LoopID = 9
		}, "unknown loop"},
		{"swapped condition successors", func(f *Function) {
			op := findEdge(f, LoopEdgeCondition)
			op.Successors[0], op.Successors[1] = op.Successors[1], op.Successors[0]
		}, "condition edge targets"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module, err := analyzedModule(t, whileLoopSource, 13)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(loopFunction(t, module, "Count"))
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Verify = %v, want %q", err, test.want)
			}
		})
	}
}
