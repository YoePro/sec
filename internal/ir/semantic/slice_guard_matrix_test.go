package semantic

import (
	"fmt"
	"strings"
	"testing"
)

// A parent validity check, normalized reborrow, child validity check and
// runtime index must compose without conflating either temporal identity or
// the spatial bounds predicate. Both authorities use the same unsigned count.
// Rules: rules/compiler/semantic_ir.md — §126.8,14–18,23–25;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§129,136.
func TestSliceGuardCompositionMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/mutable=%v", width, mutable), func(t *testing.T) {
				m := sliceGuardMatrixModule(mutable)
				m.PointerWidthBits = width
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				other := sliceGuardMatrixModule(mutable)
				other.PointerWidthBits = width
				if Format(m) != Format(other) {
					t.Fatal("fresh construction changed slice identities or metadata order")
				}
				for _, unwanted := range []string{"numeric.convert", "storage.load", "array.extract", "slice.len-int", "undef", "poison"} {
					if strings.Contains(Format(m), unwanted) {
						t.Fatal("unexpected operation", unwanted)
					}
				}
			})
		}
	}
}

// Each mutation starts from a verified composed graph and checks the owning
// verifier's reason. An unrelated malformed fixture cannot satisfy a negative
// test, and a valid parent check cannot validate a distinct child borrow.
// Rules: rules/compiler/semantic_ir.md — §126.8,17–18,23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§129,136.
func TestSliceGuardCompositionRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name, reason string
		change       func(*Module)
	}{
		{"parent guard substituted for child", "requires its ref.is-valid guard", func(m *Module) { m.Functions[0].Blocks[1].Operations[0].ReferenceGuard = 30 }},
		{"child guard validates parent SSA", "requires its ref.is-valid guard", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Operands[0] = 1 }},
		{"reborrow loses parent guard", "requires its ref.is-valid guard", func(m *Module) { m.Functions[0].Blocks[6].Operations[2].ReferenceGuard = 0 }},
		{"missing normalization proof", "requires range proof provenance", func(m *Module) { m.Functions[0].Blocks[6].Operations[1].SliceRange.ProofKind = "" }},
		{"child epoch rewritten", "same source borrow and epoch", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.EpochDependency++ }},
		{"stale failure becomes spatial", "must terminate with fail.reference-generation", func(m *Module) {
			m.Functions[0].Blocks[4].Operations[0] = Operation{Kind: OpBoundsFailure, ArrayOperation: "slice-index"}
		}},
		{"wrong source at spatial predicate", "does not match its slice/index bounds guard", func(m *Module) {
			predicate := &m.Functions[0].Blocks[1].Operations[0]
			predicate.Operands[0], predicate.ReferenceGuard = 1, 30
		}},
		{"normalized start replaced by length", "same normalized range start/length tuple", func(m *Module) { m.Functions[0].Blocks[6].Operations[2].Operands[1] = 33 }},
		{"read derived Place after parent end", "used after its resolved borrow end", func(m *Module) {
			block := m.Functions[0].Blocks[2]
			end := block.Operations[len(block.Operations)-2]
			block.Operations = append(block.Operations[:1], append([]Operation{end}, block.Operations[1:len(block.Operations)-2]...)...)
			block.Operations = append(block.Operations, Operation{Kind: OpReturn, Operands: []ValueID{6}})
		}},
	}
	for _, width := range []uint16{32, 64} {
		for _, test := range tests {
			t.Run(fmt.Sprintf("%d/%s", width, test.name), func(t *testing.T) {
				m := sliceGuardMatrixModule(false)
				m.PointerWidthBits = width
				if err := Verify(m); err != nil {
					t.Fatal("invalid baseline", err)
				}
				test.change(m)
				if err := Verify(m); err == nil || !strings.Contains(err.Error(), test.reason) {
					t.Fatalf("want %q, got %v", test.reason, err)
				}
			})
		}
	}
}

// Assemble separate parent and child reference guards around a full-range
// reborrow and guarded element read. EpochDependency is a symbolic identity,
// not a runtime epoch counter or a chosen physical representation.
// Rules: rules/compiler/semantic_ir.md — §126.8,14–18,23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§43–46,129,136.
func sliceGuardMatrixModule(mutable bool) *Module {
	m := sliceIndexModule(mutable, true, true, "read")
	fn := m.Functions[0]
	uintType := m.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	parent := cloneReferenceMetadata(fn.Parameters[0].Reference)
	child := cloneReferenceMetadata(parent)
	child.Borrow, child.Lifetime, child.SourceBorrow = 2, 2, parent.Borrow
	slice := fn.Parameters[0].Value.Type
	boolean := fn.Blocks[0].Operations[0].Results[0].Type
	parentGuard := fn.Blocks[0].Operations[0]
	parentGuard.Operands = []ValueID{1}
	parentGuard.Results = []Value{{ID: 30, Type: boolean, Ownership: OwnershipImmediate}}
	parentGuard.Reference = parent
	fn.Blocks[0].Operations[0].Operands[0] = 34
	fn.Blocks[0].Operations[0].Reference = cloneReferenceMetadata(child)
	fn.Blocks[1].Operations[0].Operands[0] = 34
	fn.Blocks[2].Operations[0].Operands[0] = 34
	fn.Blocks[2].Operations[0].Reference = cloneReferenceMetadata(child)
	work := fn.Blocks[2]
	work.Operations = append(work.Operations[:len(work.Operations)-1],
		Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{34}, Reference: cloneReferenceMetadata(child)},
		Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{1}, Reference: cloneReferenceMetadata(parent)},
		Operation{Kind: OpReturn, Operands: []ValueID{6}})
	kind, ownership := OpSliceReborrowShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpSliceReborrowMutable, OwnershipMutableReference
	}
	fn.Entry = 10
	fn.Blocks = append(fn.Blocks,
		&Block{ID: 10, Operations: []Operation{parentGuard, {Kind: OpCondBranch, Operands: []ValueID{30}, Successors: []BranchTarget{{Block: 11}, {Block: 3}}}}},
		&Block{ID: 11, Operations: []Operation{
			{Kind: OpSliceLength, Operands: []ValueID{1}, Results: []Value{{ID: 31, Type: uintType, Ownership: OwnershipImmediate}}, ReferenceGuard: 30},
			{Kind: OpSliceRangeNormalize, Operands: []ValueID{31}, Results: []Value{{ID: 32, Type: uintType, Ownership: OwnershipImmediate}, {ID: 33, Type: uintType, Ownership: OwnershipImmediate}}, SliceRange: &SliceRangeMetadata{ProofKind: SliceRangeProofOpenBound}},
			{Kind: kind, Operands: []ValueID{1, 32, 33}, Results: []Value{{ID: 34, Type: slice, Ownership: ownership}}, Reference: cloneReferenceMetadata(child), ReferenceGuard: 30},
			{Kind: OpBranch, Successors: []BranchTarget{{Block: 1}}},
		}},
	)
	return m
}
