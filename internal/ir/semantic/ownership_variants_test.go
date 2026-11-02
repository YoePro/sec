package semantic

import (
	"strings"
	"testing"
)

// A moved payload ends the local owner on its own path only, before the payload
// is projected, and the binding is registered in a scope that ends with the arm.
// A fresh subject has no registration; an arm that does not take it destroys it.
// Rules: rules/control-flow/flowcontrol_match.md — §§ 13, 18;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 14, 63, 111.
func TestPackage17LowersOwnedPayloadBindings(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	kinds := func(name string, block int) []OpKind {
		var result []OpKind
		for _, op := range functionNamed(t, module, name).Blocks[block].Operations {
			result = append(result, op.Kind)
		}
		return result
	}
	before := func(list []OpKind, first, second OpKind) bool {
		seen := false
		for _, kind := range list {
			seen = seen || kind == first
			if kind == second {
				return seen
			}
		}
		return false
	}
	local := functionNamed(t, module, "MatchLocal")
	found := false
	for index := range local.Blocks {
		list := kinds("MatchLocal", index)
		for _, kind := range list {
			if kind == OpResultUnwrapOk {
				found = true
				if !before(list, OpCleanupCancel, OpResultUnwrapOk) || !before(list, OpResultUnwrapOk, OpCleanupTrackOwned) || !before(list, OpCleanupTrackOwned, OpCleanupRunScope) {
					t.Fatalf("payload move order: %v", list)
				}
			}
		}
	}
	if !found {
		t.Fatalf("no moved Ok payload in:\n%s", Format(module))
	}
	fresh := Format(module)
	if !strings.Contains(fresh, "temporary-end") {
		t.Fatalf("fresh None arm does not end its subject:\n%s", fresh)
	}
	for _, plan := range module.DestructionPlans {
		if strings.Contains(plan.ID, "Option") && (plan.Kind != "active-variant" || len(plan.Variants) != 1 || plan.Variants[0].Variant != 0) {
			t.Fatalf("Option plan covers Some only: %+v", plan)
		}
		if strings.Contains(plan.ID, "Result[main:struct:Handle") && (len(plan.Variants) != 1 || plan.Variants[0].Variant != 0) {
			t.Fatalf("Result plan covers its non-trivial Ok only: %+v", plan)
		}
	}
}

// A guarded arm commits its move only once the guard holds: the guard's block
// reads a candidate, ends nothing, and the arm body cancels and takes the payload.
// A candidate that escapes the guard is rejected.
// Rules: rules/control-flow/flowcontrol_match.md — § 17.
func TestPackage17GuardedPayloadCommitsInTheBody(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	function := functionNamed(t, module, "GuardLocal")
	var candidate, guard, commit bool
	for _, block := range function.Blocks {
		reads, cancels := false, false
		for _, op := range block.Operations {
			reads = reads || op.Kind == OpPlaceRead && op.PlaceReadMode == PlaceReadCandidate
			cancels = cancels || op.Kind == OpCleanupCancel
			if op.Kind == OpUnionUnwrapPayload && len(op.PayloadActions) == 1 && op.PayloadActions[0] == UnionPayloadMove {
				commit = commit || cancels
			}
		}
		if reads {
			candidate = true
			guard = !cancels
		}
	}
	if !candidate || !guard || !commit {
		t.Fatalf("candidate=%v guard-ends-nothing=%v commit=%v:\n%s", candidate, guard, commit, Format(module))
	}
	for _, block := range function.Blocks {
		for index := range block.Operations {
			op := &block.Operations[index]
			if op.Kind == OpPlaceRead && op.PlaceReadMode == PlaceReadCandidate {
				candidateValue := op.Results[0].ID
				for _, other := range function.Blocks {
					for use := range other.Operations {
						if other.Operations[use].Kind == OpStructExtractField && other.Operations[use].Operands[0] == candidateValue {
							saved := other.Operations[use].Kind
							other.Operations[use].Kind = OpOwnDiscard
							blocks := blockMapOf(function)
							preds := map[BlockID][]BlockID{}
							for _, source := range function.Blocks {
								for _, sourceOp := range source.Operations {
									for _, successor := range sourceOp.Successors {
										preds[successor.Block] = append(preds[successor.Block], source.ID)
									}
								}
							}
							if err := verifyUnionPayloadPlaces(function, blocks, nil, dominators(function.Entry, blocks, preds)); err == nil || !strings.Contains(err.Error(), "guard candidate") {
								t.Fatalf("escaping candidate accepted: %v", err)
							}
							other.Operations[use].Kind = saved
							return
						}
					}
				}
			}
		}
	}
	t.Fatal("no candidate field read found")
}

func blockMapOf(function *Function) map[BlockID]*Block {
	blocks := map[BlockID]*Block{}
	for _, block := range function.Blocks {
		blocks[block.ID] = block
	}
	return blocks
}
