package semantic

import (
	"fmt"
	"strings"
	"testing"
)

// Canonical reference facts survive same-holder CFG forwarding, including an
// anchored loop. Both authorities retain their own temporal guard requirements;
// verification must not mutate the represented IR or depend on block order.
// Rules: rules/compiler/semantic_ir.md — §§66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,63,83–86.
func TestSliceReferenceFactsFollowCFG(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/mutable=%v/dynamic=%v", width, mutable, dynamic), func(t *testing.T) {
					m := forwardedSliceLengthModule(mutable, dynamic)
					m.PointerWidthBits = width
					before := Format(m)
					if err := Verify(m); err != nil {
						t.Fatal(err)
					}
					if Format(m) != before {
						t.Fatal("verification mutated CFG reference facts")
					}
					fn := m.Functions[0]
					for i, j := 0, len(fn.Blocks)-1; i < j; i, j = i+1, j-1 {
						fn.Blocks[i], fn.Blocks[j] = fn.Blocks[j], fn.Blocks[i]
					}
					if err := Verify(m); err != nil {
						t.Fatal("block order changed reference proof", err)
					}
				})
			}
			m := forwardedSliceLengthModule(mutable, false)
			m.PointerWidthBits = width
			fn := m.Functions[0]
			boolean := m.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
			fn.Parameters = append(fn.Parameters, Parameter{Name: "again", Value: Value{ID: 40, Type: boolean, Ownership: OwnershipImmediate}})
			fn.Blocks[1].Operations[1] = Operation{Kind: OpCondBranch, Operands: []ValueID{40}, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{20}}, {Block: 4}}}
			fn.Blocks = append(fn.Blocks, &Block{ID: 4, Operations: []Operation{{Kind: OpReturn, Operands: []ValueID{3}}}})
			if err := Verify(m); err != nil {
				t.Fatal("anchored loop lost facts", err)
			}
		}
	}
}

// Every incoming edge participates. An incompatible alternative cannot be
// hidden behind a valid first predecessor, nor can forwarding remove a dynamic
// source's guard or replace its epoch with the other branch's epoch.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,83–86.
func TestSliceReferenceCFGRejectsContradictions(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		change       func(*Module)
	}{
		{"missing dynamic guard", "requires its ref.is-valid guard", func(m *Module) { m.Functions[0].Blocks[2].Operations[0].ReferenceGuard = 0 }},
		{"changed guard epoch", "same source borrow and epoch", func(m *Module) { m.Functions[0].Blocks[1].Operations[0].Reference.EpochDependency++ }},
		{"mixed incoming facts", "no canonical reference facts", func(m *Module) {
			fn := m.Functions[0]
			other := fn.Parameters[0]
			other.Name, other.Value.ID = "other", 50
			other.Reference = cloneReferenceMetadata(other.Reference)
			other.Reference.OriginStorages = []uint32{2}
			fn.Parameters = append(fn.Parameters, other)
			boolean := m.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
			fn.Parameters = append(fn.Parameters, Parameter{Name: "choose", Value: Value{ID: 40, Type: boolean, Ownership: OwnershipImmediate}})
			fn.Blocks[0].Operations[0] = Operation{Kind: OpCondBranch, Operands: []ValueID{40}, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{1}}, {Block: 1, Arguments: []ValueID{50}}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := forwardedSliceLengthModule(false, true)
			if err := Verify(m); err != nil {
				t.Fatal("baseline", err)
			}
			tc.change(m)
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("want %s: %v", tc.reason, err)
			}
		})
	}
}

// A P15 reference derived from a bounded element Place cannot invent a
// different storage/epoch/pin/address contract, even though its local operation
// shape and standalone metadata remain valid.
// Rules: rules/compiler/semantic_ir.md — §126.17–19,23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,85–86,116.
func TestSliceElementBorrowPreservesGrantingFacts(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			use := "shared-borrow"
			if mutable {
				use = "mutable-borrow"
			}
			for _, tc := range []struct {
				name   string
				change func(*ReferenceMetadata)
			}{
				{"same borrow identity", func(f *ReferenceMetadata) { f.Borrow = 1 }},
				{"downgraded validity", func(f *ReferenceMetadata) {
					f.Validity, f.ProofProvenance = ReferenceProven, "lexical"
					f.InvalidationDomain, f.EpochDependency = 0, 0
				}},
				{"wrong storage", func(f *ReferenceMetadata) { f.OriginStorages[0]++ }},
				{"wrong epoch", func(f *ReferenceMetadata) { f.EpochDependency++ }},
				{"wrong domain", func(f *ReferenceMetadata) { f.InvalidationDomain++ }},
				{"changed pin", func(f *ReferenceMetadata) { f.Pinned = !f.Pinned }},
				{"changed stability", func(f *ReferenceMetadata) { f.AddressStability = "other-stability" }},
				{"changed address space", func(f *ReferenceMetadata) { f.AddressSpace = AddressSpaceMMIO }},
				{"changed provenance", func(f *ReferenceMetadata) { f.Provenance = "different-origin" }},
			} {
				t.Run(fmt.Sprintf("%d/mutable=%v/%s", width, mutable, tc.name), func(t *testing.T) {
					m := sliceIndexModule(mutable, true, true, use)
					m.PointerWidthBits = width
					if err := Verify(m); err != nil {
						t.Fatal("baseline", err)
					}
					for _, block := range m.Functions[0].Blocks {
						for i := range block.Operations {
							op := &block.Operations[i]
							if op.Kind == OpReferenceBorrowShared || op.Kind == OpReferenceBorrowMutable {
								tc.change(op.Reference)
							}
						}
					}
					if err := Verify(m); err == nil || !strings.Contains(err.Error(), "slice element borrow") {
						t.Fatal("forged element borrow accepted or wrong owner", err)
					}
				})
			}
		}
	}
}

// A rootless forwarding cycle cannot borrow the facts from a separate valid
// predecessor of a downstream join. This exercises the fixed-point solver
// directly, without claiming the rootless graph is valid source-generated IR.
// Rules: rules/compiler/semantic_ir.md — §§66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,83–86.
func TestSliceReferenceUnknownCyclesStayUnknown(t *testing.T) {
	m := sliceLengthModule(false, false)
	fn := m.Functions[0]
	parameter := fn.Parameters[0].Value
	arg := func(id ValueID) Value { result := parameter; result.ID = id; return result }
	fn.Blocks = []*Block{
		{ID: 0, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3, Arguments: []ValueID{1}}}}}},
		{ID: 1, Parameters: []Value{arg(20)}, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 2, Arguments: []ValueID{20}}}}}},
		{ID: 2, Parameters: []Value{arg(21)}, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{40}, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{21}}, {Block: 3, Arguments: []ValueID{21}}}}}},
		{ID: 3, Parameters: []Value{arg(22)}, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 4, Arguments: []ValueID{22}}}}}},
		{ID: 4, Parameters: []Value{arg(23)}, Operations: []Operation{{Kind: OpReturn}}},
	}
	blocks := map[BlockID]*Block{}
	for _, block := range fn.Blocks {
		blocks[block.ID] = block
	}
	sources := map[ValueID]*ReferenceMetadata{1: fn.Parameters[0].Reference}
	resolveForwardedSliceFacts(m.Types, fn, blocks, sources)
	for _, id := range []ValueID{20, 21, 22, 23} {
		if sources[id] != nil {
			t.Fatal("unknown cycle acquired facts", id)
		}
	}
	// Remove the unknown incoming edge: the valid source now anchors both
	// downstream parameters, and each published storage list is detached.
	fn.Blocks[2].Operations[0].Successors = fn.Blocks[2].Operations[0].Successors[:1]
	resolveForwardedSliceFacts(m.Types, fn, blocks, sources)
	if sources[22] == nil || sources[23] == nil {
		t.Fatal("known chain lost source facts")
	}
	sources[22].OriginStorages[0]++
	if sources[1].OriginStorages[0] != 1 || sources[23].OriginStorages[0] != 1 {
		t.Fatal("forwarded facts share mutable storage lists")
	}
}

// Forward the same bounded holder to a fresh block argument before any
// observation/validity guard; no new borrow or physical descriptor is created.
// Rules: rules/compiler/semantic_ir.md — §§66,126.23.
func forwardedSliceLengthModule(mutable, dynamic bool) *Module {
	m := sliceLengthModule(mutable, dynamic)
	forwardSliceParameter(m)
	return m
}

// forwardSliceParameter adds a single canonical holder-forwarding edge before
// the first block of the length/transfer fixture, preserving its operations.
// Rules: rules/compiler/semantic_ir.md — §§66,126.23.
func forwardSliceParameter(m *Module) {
	fn := m.Functions[0]
	parameter := fn.Parameters[0].Value
	parameter.ID = 20
	fn.Blocks[0].Parameters = []Value{parameter}
	for _, block := range fn.Blocks {
		for i := range block.Operations {
			op := &block.Operations[i]
			for j, id := range op.Operands {
				if id == 1 {
					op.Operands[j] = 20
				}
			}
		}
	}
	fn.Entry = 0
	fn.Blocks = append([]*Block{{ID: 0, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{1}}}}}}}, fn.Blocks...)
}

// Non-observing copy/move/end consumers retain their complete identities after
// CFG forwarding; a dynamic epoch introduces no backing observation here.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,63,86.
func TestSliceReferenceForwardedTransfers(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/mutable=%v/dynamic=%v", width, mutable, dynamic), func(t *testing.T) {
					m := sliceTransferModule(mutable, dynamic)
					m.PointerWidthBits = width
					forwardSliceParameter(m)
					if err := Verify(m); err != nil {
						t.Fatal("forwarded transfer/end", err)
					}
				})
			}
		}
	}
}
