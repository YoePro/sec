package semantic

import (
	"strings"
	"testing"
)

// Copy/move/end operate on holders, preserve the bounded reference value and
// complete P15 facts, and add no backing read, cleanup or epoch observation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,45,63.
func TestSliceHolderTransfers(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				module := sliceTransferModule(mutable, dynamic)
				module.PointerWidthBits = width
				if err := Verify(module); err != nil {
					t.Fatal(err)
				}
				text := Format(module)
				kind := OpSliceCopyShared
				if mutable {
					kind = OpSliceMoveMutable
				}
				if !strings.Contains(text, string(kind)+" %1") || !strings.Contains(text, "ref.end-borrow %2") || Format(module) != text {
					t.Fatalf("holder printing:\n%s", text)
				}
				for _, forbidden := range []string{"place.read", "storage.load", "array.extract", "slice.range-", "ref.is-valid", "fail.reference-generation"} {
					if strings.Contains(text, forbidden) {
						t.Fatalf("holder transfer introduced %s", forbidden)
					}
				}
			}
		}
	}
}

// Ending either shared holder leaves its independent copied holder usable;
// no borrow counter, element destruction or backing-storage release is added.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§40,63.
func TestSharedSliceCopyHasIndependentHolderEnd(t *testing.T) {
	for _, endOriginal := range []bool{false, true} {
		module := sliceTransferModule(false, false)
		fn := module.Functions[0]
		uintType := module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
		fn.ReturnType = uintType
		ended, remaining := ValueID(2), ValueID(1)
		facts := cloneReferenceMetadata(fn.Blocks[0].Operations[0].Reference)
		if endOriginal {
			ended, remaining = 1, 2
			facts = cloneReferenceMetadata(fn.Parameters[0].Reference)
		}
		fn.Blocks[0].Operations = []Operation{
			fn.Blocks[0].Operations[0],
			{Kind: OpReferenceEndBorrow, Operands: []ValueID{ended}, Reference: facts},
			{Kind: OpSliceLength, Operands: []ValueID{remaining}, Results: []Value{{ID: 3, Type: uintType, Ownership: OwnershipImmediate}}},
			{Kind: OpReturn, Operands: []ValueID{3}},
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
	}
}

// Explicit mutable moves invalidate the old holder along CFG paths, and
// copy/move/end cannot change borrow/lifetime/epoch/storage or authority.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,63.
func TestSliceTransfersRejectInvalidOwnershipAndFacts(t *testing.T) {
	tests := []struct {
		name    string
		mutable bool
		change  func(*Module)
	}{
		{"copy mutable slice", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Kind = OpSliceCopyShared }},
		{"move shared slice", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Kind = OpSliceMoveMutable }},
		{"wrong result authority", true, func(m *Module) {
			id, _ := m.Types.InternSlice(1, false)
			m.Functions[0].Blocks[0].Operations[0].Results[0].Type = id
		}},
		{"wrong result ownership", true, func(m *Module) {
			m.Functions[0].Blocks[0].Operations[0].Results[0].Ownership = OwnershipSharedReference
		}},
		{"new borrow", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.Borrow++ }},
		{"new lifetime", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.Lifetime++ }},
		{"new epoch", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.EpochDependency++ }},
		{"new storage", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.OriginStorages[0]++ }},
		{"new source borrow", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.SourceBorrow++ }},
		{"range rewrite", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].SliceRange = &SliceRangeMetadata{} }},
		{"new observation", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].ReferenceGuard = 99 }},
		{"wrong end lifetime", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[1].Reference.Lifetime++ }},
		{"wrong parameter end", false, func(m *Module) { m.Functions[0].Blocks[0].Operations[2].Reference.Borrow++ }},
		{"discard without holder end", false, func(m *Module) {
			m.Functions[0].Blocks[0].Operations[1] = Operation{Kind: OpDiscardValue, Operands: []ValueID{2}, DiscardProvenance: DiscardExplicit}
		}},
		{"reuse moved source", true, func(m *Module) {
			fn := m.Functions[0]
			move := fn.Blocks[0].Operations[0]
			reuse := move
			reuse.Results = []Value{{ID: 3, Type: move.Results[0].Type, Ownership: OwnershipMutableReference}}
			fn.Blocks[0].Operations = []Operation{move, reuse, {Kind: OpReturn}}
		}},
		{"double end", true, func(m *Module) {
			fn := m.Functions[0]
			end := fn.Blocks[0].Operations[1]
			fn.Blocks[0].Operations = []Operation{fn.Blocks[0].Operations[0], end, end, {Kind: OpReturn}}
		}},
		{"reuse after end", false, func(m *Module) {
			fn := m.Functions[0]
			copy := fn.Blocks[0].Operations[0]
			copy.Operands = []ValueID{2}
			copy.Results = []Value{{ID: 3, Type: copy.Results[0].Type, Ownership: OwnershipSharedReference}}
			fn.Blocks[0].Operations = []Operation{fn.Blocks[0].Operations[0], fn.Blocks[0].Operations[1], copy, {Kind: OpReturn}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := sliceTransferModule(test.mutable, true)
			test.change(module)
			if err := Verify(module); err == nil {
				t.Fatal("accepted invalid slice holder transfer")
			}
		})
	}
}

// A forwarded block argument is still the original holder, rather than an
// implicit descriptor copy that could hide a use of a moved mutable slice.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39,41,63.
func TestMutableSliceMoveRejectsForwardedOldHolder(t *testing.T) {
	module := sliceTransferModule(true, false)
	fn := module.Functions[0]
	sliceType := fn.Parameters[0].Value.Type
	move := fn.Blocks[0].Operations[0]
	fn.ReturnType = sliceType
	fn.Blocks = []*Block{
		{ID: 1, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 2, Arguments: []ValueID{1}}}}}},
		{ID: 2, Parameters: []Value{{ID: 4, Type: sliceType, Ownership: OwnershipMutableReference}}, Operations: []Operation{move, {Kind: OpReturn, Operands: []ValueID{4}}}},
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "used after its mutable slice move") {
		t.Fatalf("forwarded source escaped move boundary: %v", err)
	}
}

// A loop parameter receiving the new moved holder starts another iteration;
// it does not reuse the old dynamic holder instance. This isolates lifetime
// CFG rules; complete slice block-parameter fact joins remain P16-26.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §41;
// rules/compiler/semantic_ir.md — §66.
func TestSliceMoveLifetimeBoundaryAllowsFreshLoopHolder(t *testing.T) {
	module := sliceTransferModule(true, false)
	fn := module.Functions[0]
	sliceType := fn.Parameters[0].Value.Type
	move := fn.Blocks[0].Operations[0]
	move.Operands = []ValueID{4}
	fn.Blocks = []*Block{
		{ID: 1, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 2, Arguments: []ValueID{1}}}}}},
		{ID: 2, Parameters: []Value{{ID: 4, Type: sliceType, Ownership: OwnershipMutableReference}}, Operations: []Operation{move, {Kind: OpBranch, Successors: []BranchTarget{{Block: 2, Arguments: []ValueID{2}}}}}},
	}
	blocks := map[BlockID]*Block{1: fn.Blocks[0], 2: fn.Blocks[1]}
	values := map[ValueID]Value{1: fn.Parameters[0].Value, 4: fn.Blocks[1].Parameters[0], 2: move.Results[0]}
	if err := verifyNoUseAfterEndBorrow(fn, blocks, values, module.Types); err != nil {
		t.Fatal(err)
	}
}

// Build non-observing holder transfer and canonical P15 end markers; even
// dynamic-epoch holders remain @noPanic without checking the backing epoch.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,45,63.
func sliceTransferModule(mutable, dynamic bool) *Module {
	module := sliceLengthModule(mutable, false)
	fn := module.Functions[0]
	fn.ID, fn.Name = "main::Transfer", "Transfer"
	fn.ReturnType = module.Types.Intern(Type{Kind: TypeVoid, Name: "void"})
	fn.NoPanic, fn.NoPanicSource = true, "written"
	source := fn.Parameters[0].Reference
	if dynamic {
		source.Validity, source.ProofProvenance = ReferenceDynamicEpoch, ""
		source.InvalidationDomain, source.EpochDependency = 1, 1
	}
	copied := cloneReferenceMetadata(source)
	copied.SourceBorrow = copied.Borrow
	kind, ownership := OpSliceCopyShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpSliceMoveMutable, OwnershipMutableReference
	}
	ops := []Operation{
		{Kind: kind, Operands: []ValueID{1}, Results: []Value{{ID: 2, Type: fn.Parameters[0].Value.Type, Ownership: ownership}}, Reference: copied},
		{Kind: OpReferenceEndBorrow, Operands: []ValueID{2}, Reference: cloneReferenceMetadata(copied)},
	}
	if !mutable {
		ops = append(ops, Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{1}, Reference: cloneReferenceMetadata(source)})
	}
	ops = append(ops, Operation{Kind: OpReturn})
	fn.Blocks[0].Operations = ops
	return module
}
