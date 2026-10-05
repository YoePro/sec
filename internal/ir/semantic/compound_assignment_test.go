package semantic

import (
	"errors"
	"testing"
)

// `local op= value` reads the destination once, evaluates the source once,
// applies the Sema-resolved operator with its checks, and stores once on the
// success path only; `++` is the same `+= 1`. Non-local targets remain an
// explicit boundary.
//
// Rules:
//   - rules/foundations/operators.md — "Compound assignment", "Compound arithmetic failure", "Increment and decrement aliases"
//   - rules/compiler/semantic_ir.md — § 31 "Storage operations", § 58 "Arithmetic"
func TestCompoundAssignmentBuildsCheckedReadModifyWrite(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Update(seed: int) int {
    let mut total := seed
    total += 2
    total ^= 3
    total++
    return total
}
`, 13)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}
	function := loopFunction(t, module, "Update")
	if got := countOperations(function, OpIntBinaryChecked); got != 2 {
		t.Errorf("checked additions = %d, want 2", got)
	}
	if got := countOperations(function, OpIntBitwise); got != 1 {
		t.Errorf("bitwise operations = %d, want 1", got)
	}
	if got := countOperations(function, OpArithmeticFailure); got != 2 {
		t.Errorf("arithmetic failure paths = %d, want 2", got)
	}
	if got := countOperations(function, OpStorageStore); got != 3 {
		t.Errorf("stores = %d, want one per compound assignment", got)
	}
	for _, block := range function.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpIntBinaryChecked {
				continue
			}
			if index < 2 || block.Operations[index-2].Kind != OpStorageLoad {
				t.Errorf("checked operation does not read the destination first: %+v", block.Operations)
			}
			for _, later := range block.Operations[index+1:] {
				if later.Kind == OpStorageStore {
					t.Errorf("destination is written before the overflow check")
				}
			}
		}
	}
}

func TestCompoundAssignmentToNonLocalPlaceIsAnExplicitBoundary(t *testing.T) {
	_, err := analyzedModule(t, `module main

type Counter struct {
    value: int
}

fn Bump(start: int) int {
    let mut counter := Counter { value: start }
    counter.value += 1
    return counter.value
}
`, 13)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != "compound assignment to a non-local place" {
		t.Fatalf("err = %v, want the non-local compound boundary", err)
	}
}
