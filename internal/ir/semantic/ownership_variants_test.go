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
		if strings.HasPrefix(plan.ID, "destroy::union:Option") && (plan.Kind != "active-variant" || len(plan.Variants) != 1 || plan.Variants[0].Variant != 0) {
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

// A binding reassigned in a nested scope or loop has a path-dependent live
// incarnation: every end of it cancels each incarnation where active, even an
// end built before the assignment that a loop reaches again. A binding assigned
// only in its own block keeps one plain cancel.
// Rules: rules/memory/destruction.md — §§ 9.1, 13; rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 38, 55.
func TestPackage17PathDependentReassignment(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	cancels := func(name string) (plain, ifActive int) {
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpCleanupCancel {
					if op.Cleanup.IfActive {
						ifActive++
					} else {
						plain++
					}
				}
			}
		}
		return
	}
	if plain, ifActive := cancels("MoveAndReinitializeInLoop"); plain != 0 || ifActive != 2 {
		t.Errorf("loop move: %d plain and %d if-active cancels, want 0 and 2", plain, ifActive)
	}
	if plain, ifActive := cancels("ReassignThenMove"); plain != 0 || ifActive < 2 {
		t.Errorf("move after a nested reassignment: %d plain and %d if-active cancels", plain, ifActive)
	}
	if plain, ifActive := cancels("Forward"); plain == 0 || ifActive != 0 {
		t.Errorf("unassigned binding: %d plain and %d if-active cancels, want only plain", plain, ifActive)
	}
}

// A moved stored field leaves its owner registered: no cleanup.cancel follows it,
// the owner lives in storage even when immutable, and a reinitialized or replaced
// field stays under the owner's registration with no cleanup identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 12–13, 58–59, 64.
func TestPackage17FieldMovesKeepTheOwnerRegistered(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	count := func(name string, kind OpKind) int {
		n := 0
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if op.Kind == kind {
					n++
				}
			}
		}
		return n
	}
	if count("FieldMove", OpCleanupCancel) != 0 || count("FieldMove", OpOwnMoveFromPlace) != 1 || count("FieldMove", OpStorageDeclare) != 1 {
		t.Errorf("an immutable owner's field move:\n%s", Format(module))
	}
	for _, block := range functionNamed(t, module, "FieldReplaceAndReinitialize").Blocks {
		for _, op := range block.Operations {
			if (op.Kind == OpOwnReplacePlace || op.Kind == OpOwnInitializePlace) && op.Ownership.OldCleanup != 0 {
				t.Errorf("a field assignment changed cleanup identities: %+v", op.Ownership)
			}
		}
	}
	if count("FieldReplaceAndReinitialize", OpOwnReplacePlace) != 1 || count("FieldReplaceAndReinitialize", OpPlaceWrite) != 1 {
		t.Errorf("field replacement and trivial write:\n%s", Format(module))
	}
}

// `if x is Some(y)` without an else whose consequence falls through branches to
// the continuation once; the second branch it used to emit failed verification
// even for a trivially copyable payload (package 12).
// Rules: rules/control-flow/flowcontrol_if.md — § 12.
func TestOptionPresenceWithoutElseBranchesOnce(t *testing.T) {
	for _, maxPackage := range []uint8{12, 17} {
		module, err := analyzedModule(t, `module main

fn Fresh() Option[int] {
    return Some(1)
}

fn Use(number: int) void {
}

fn T() void {
    let option := Fresh()
    if option is Some(number) {
        Use(number)
    }
}

fn main() int {
    return 0
}
`, maxPackage)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("package %d: %v", maxPackage, err)
		}
	}
}

// Replacing a whole owner after fields moved destroys only the remaining fields:
// a destroy-place skips the moved paths, or acts only on what is still owned when
// a field moved on some paths, before the new value initializes the Place.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 12, 31, 58; rules/memory/destruction.md — § 13.
func TestPackage17ReplacementAfterFieldMoves(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	destroy := func(name string) *OwnershipFacts {
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpOwnDestroyPlace {
					return op.Ownership
				}
			}
		}
		t.Fatalf("%s has no destroy-place:\n%s", name, Format(module))
		return nil
	}
	if facts := destroy("ReplaceAfterFieldMoves"); len(facts.Skipped) != 2 || facts.IfOwned {
		t.Errorf("two moved fields: %+v", facts)
	}
	if facts := destroy("ReplaceAfterNestedFieldMove"); len(facts.Skipped) != 1 || len(facts.Skipped[0]) != 2 {
		t.Errorf("nested moved field: %+v", facts)
	}
	if facts := destroy("ReplaceAfterConditionalFieldMove"); !facts.IfOwned || len(facts.Skipped) != 0 {
		t.Errorf("conditionally moved field: %+v", facts)
	}
	if facts := destroy("ReplaceAfterPayloadMove"); len(facts.Skipped) != 1 || facts.Skipped[0].String() != "field#0" {
		t.Errorf("moved payload skips its union field: %+v", facts)
	}
}

// Temporaries end with their full expression in reverse creation order, and a
// match subject the selected arm leaves ends with the arm: on a return after the
// arm's own locals (run-scope) and before the function's (run-function).
// Rules: rules/memory/destruction.md — § 8(3), § 8(8), § 8(9).
func TestPackage17TemporariesEndWithTheirFullExpression(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	destroys := 0
	for _, block := range functionNamed(t, module, "Temporaries").Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpOwnDestroyValue && op.Ownership.Cause == "temporary-end" {
				destroys++
			}
		}
	}
	if destroys != 6 {
		t.Errorf("Temporaries destroys %d temporaries, want 6:\n%s", destroys, Format(module))
	}
	found := false
	for _, block := range functionNamed(t, module, "LeftSubjectReturns").Blocks {
		var kinds []OpKind
		for _, op := range block.Operations {
			kinds = append(kinds, op.Kind)
		}
		for index := 0; index+3 < len(kinds); index++ {
			if kinds[index] == OpCleanupRunScope && kinds[index+1] == OpOwnDestroyValue && kinds[index+2] == OpCleanupRunFunction && kinds[index+3] == OpReturn {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("a return from an arm must clean its locals, then the subject, then the function:\n%s", Format(module))
	}
}

// Result.Ok()/Err() consume the receiver: the retained payload moves into Some,
// and the inactive payload ends by its plan before None (destruction.md § 16.4).
// Rules: rules/errors/errorhandling.md — § 6.1; rules/memory/destruction.md — § 16.4.
func TestPackage17ResultProjections(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	count := func(name string, match func(Operation) bool) int {
		n := 0
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if match(op) {
					n++
				}
			}
		}
		return n
	}
	constructs := func(op Operation) bool { return op.Kind == OpUnionConstruct }
	for _, name := range []string{"ProjectTrivial", "ProjectOwned", "ProjectDropsOwnedError", "ProjectError", "ProjectField"} {
		if count(name, constructs) != 2 {
			t.Errorf("%s builds Some and None once each:\n%s", name, Format(module))
		}
	}
	dropped := func(op Operation) bool {
		return op.Kind == OpOwnDestroyValue && op.Ownership.Cause == "compiler-generated"
	}
	if count("ProjectDropsOwnedError", dropped) != 1 || count("ProjectOwned", dropped) != 0 {
		t.Errorf("only an inactive payload that needs destruction is destroyed:\n%s", Format(module))
	}
}

// A fresh subject whose arms test or take a payload only a Place reaches is held
// in compiler storage: arms that leave it destroy it in place, the arm that takes
// an error variant's payload moves it out through the Err payload Place.
// Rules: rules/control-flow/flowcontrol_match.md — §§ 13, 28; rules/memory/destruction.md — § 8(8).
func TestPackage17MaterializedMatchSubjects(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	count := func(name string, kind OpKind) int {
		n := 0
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if op.Kind == kind {
					n++
				}
			}
		}
		return n
	}
	if count("LostPayload", OpStorageDeclare) != 1 || count("LostPayload", OpOwnMoveFromPlace) != 1 || count("LostPayload", OpOwnDestroyPlace) != 3 {
		t.Errorf("LostPayload must take one payload and end the subject in the three other arms:\n%s", Format(module))
	}
	if count("MessageFresh", OpOwnMoveFromPlace) != 1 || count("MessageFresh", OpOwnDestroyPlace) != 1 {
		t.Errorf("MessageFresh must move the named-field payload or end the subject:\n%s", Format(module))
	}
}

// A try handler owns the error it receives: a binding takes the error or its
// payload, and a handler that does not destroys the error on every way out.
// Rules: rules/errors/errorhandling.md — §§ 15.3, 20; rules/memory/destruction.md — § 8(8).
func TestPackage17TryHandlersEndOwnedErrors(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/ownership_variants.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	count := func(name string, match func(Operation) bool) int {
		n := 0
		for _, block := range functionNamed(t, module, name).Blocks {
			for _, op := range block.Operations {
				if match(op) {
					n++
				}
			}
		}
		return n
	}
	moves := func(op Operation) bool {
		return op.Kind == OpUnionUnwrapPayload && len(op.PayloadActions) == 1 && op.PayloadActions[0] == UnionPayloadMove
	}
	ends := func(op Operation) bool { return op.Kind == OpOwnDestroyValue && op.Ownership.Cause == "temporary-end" }
	if count("HandlerTakesPayload", moves) != 1 || count("HandlerTakesPayload", ends) != 1 {
		t.Errorf("HandlerTakesPayload takes one payload and ends the error in the value handler:\n%s", Format(module))
	}
	if count("HandlerLeavesError", ends) != 1 || count("HandlerInLoop", ends) != 1 {
		t.Errorf("a handler that leaves the error destroys it:\n%s", Format(module))
	}
}
