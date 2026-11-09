package semantic

import (
	"os"
	"testing"
)

// TestMatchOwnedResultsTransferBeforeCleanup preserves the arm's result while
// unrelated arm locals and the moved-from payload registration are cleaned up.
// Rules: rules/control-flow/flowcontrol_match.md §21.2(1)-(3);
// rules/memory/destruction.md §§6.2,8; rules/mlir/packages/sec-mlir-dialect_package17.md §§38,40,55.
func TestMatchOwnedResultsTransferBeforeCleanup(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/match_owned_results.sec")
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
	for _, name := range []string{"Local", "Explicit", "Payload", "Guarded"} {
		fn := loopFunction(t, module, name)
		ordered := 0
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind != OpOwnMove || len(op.Results) != 1 {
					continue
				}
				cancel, cleanup, branch := -1, -1, -1
				for next := index + 1; next < len(block.Operations); next++ {
					following := block.Operations[next]
					if following.Kind == OpCleanupCancel && cancel < 0 {
						cancel = next
					}
					if following.Kind == OpCleanupRunScope && cleanup < 0 {
						cleanup = next
					}
					if following.Kind == OpBranch && following.MatchStage == "body-exit" {
						for _, target := range following.Successors {
							if len(target.Arguments) == 1 && target.Arguments[0] == op.Results[0].ID {
								branch = next
							}
						}
					}
				}
				if branch < 0 {
					continue
				}
				if cancel <= index || (cleanup >= 0 && (cleanup <= cancel || cleanup >= branch)) {
					t.Fatalf("%s: result transfer/cancellation is not before arm cleanup and merge", name)
				}
				ordered++
			}
		}
		if ordered == 0 {
			t.Fatalf("%s has no protected owned result handed to a merge", name)
		}
	}
}
