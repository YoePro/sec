package semantic

import (
	"os"
	"testing"
)

// TestStatementMatchOwnedResults destroys fresh arm results before continuing
// while payload transfers, arm locals and full-expression temporaries retain
// their ordinary explicit ownership and cleanup operations.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/control-flow/discard.md §§9-10; rules/memory/destruction.md §8.
func TestStatementMatchOwnedResults(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/match_statement_owned.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"Returned": 2, "Payload": 1, "ArmBlock": 1, "Temporary": 0, "Tried": 1} {
		fn := loopFunction(t, module, name)
		if got := countOperations(fn, OpOwnDiscard); got != want {
			t.Fatalf("%s: got %d owned discards, want %d", name, got, want)
		}

		if name == "Temporary" {
			found := false
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if (op.Kind == OpOwnDestroyPlace || op.Kind == OpOwnDestroyValue) && op.Ownership != nil && op.Ownership.Cause == "temporary-end" {
						found = true
					}
				}
			}
			if !found {
				t.Fatal("borrowed argument temporary has no destruction at its full-expression end")
			}
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind != OpOwnDiscard {
					continue
				}
				if op.Ownership == nil || op.Ownership.DiscardOrigin != "implicit-call-result" || op.Ownership.DestructionPlan == "" {
					t.Fatalf("%s: missing terminal result ownership: %+v", name, op)
				}

				if name == "ArmBlock" || name == "Payload" {
					scopeEnd := false
					for _, following := range block.Operations[index+1:] {
						if following.Kind == OpCleanupRunScope {
							scopeEnd = true
						}
					}
					if !scopeEnd {
						t.Fatalf("%s: missing arm cleanup after result discard", name)
					}
				}
				// The discard belongs to the selected arm, before the branch
				// that reaches the continuation, not in the merge block.
				if block.Operations[len(block.Operations)-1].Kind != OpBranch || index == len(block.Operations)-1 {
					t.Fatalf("%s: discard not before arm continuation", name)
				}
			}
		}
	}
}
