package semantic

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// Exact source/index and normalized tuple identities survive CFG transport.
// Temporal and spatial guards must still dominate their actual successful
// edges; neither pointer layout nor source range inference is introduced.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.7–8,126.17–18,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86.
func TestSliceCFGForwardedProofs(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				for _, operation := range []string{"index", "reborrow"} {
					t.Run(fmt.Sprintf("%d/mutable=%v/dynamic=%v/%s", width, mutable, dynamic, operation), func(t *testing.T) {
						m, kind := sliceIndexModule(mutable, dynamic, true, "read"), OpSliceElementPlace
						if operation == "reborrow" {
							m = sliceBorrowModule(true, mutable, dynamic)
							kind = OpSliceReborrowShared
							if mutable {
								kind = OpSliceReborrowMutable
							}
						}
						m.PointerWidthBits = width
						if err := Verify(m); err != nil {
							t.Fatal("baseline", err)
						}
						forwardSliceProofOperation(t, m, kind)
						before := Format(m)
						if err := Verify(m); err != nil {
							t.Fatal("forwarded proof", err)
						}
						if Format(m) != before {
							t.Fatal("verification rewrote input SSA")
						}
						fn := m.Functions[0]
						for i, j := 0, len(fn.Blocks)-1; i < j; i, j = i+1, j-1 {
							fn.Blocks[i], fn.Blocks[j] = fn.Blocks[j], fn.Blocks[i]
						}
						if err := Verify(m); err != nil {
							t.Fatal("block order changed proof", err)
						}
					})
				}
			}
		}
	}
}

// Equal reference metadata does not make different slice SSA values equal.
// Swapped tuple components and bypasses retain their own owning diagnostics.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86.
func TestSliceCFGRejectsForgedForwarding(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		kind         OpKind
		change       func(*Module, *Block, *BranchTarget)
	}{
		{"different slice", "does not match its slice/index bounds guard", OpSliceElementPlace, func(m *Module, _ *Block, edge *BranchTarget) {
			p := m.Functions[0].Parameters[0]
			p.Name = "other"
			p.Value.ID = 200
			p.Reference = cloneReferenceMetadata(p.Reference)
			m.Functions[0].Parameters = append(m.Functions[0].Parameters, p)
			edge.Arguments[0] = 200
		}},
		{"different index", "does not match its slice/index bounds guard", OpSliceElementPlace, func(m *Module, _ *Block, edge *BranchTarget) {
			p := m.Functions[0].Parameters[1]
			p.Name = "other"
			p.Value.ID = 200
			m.Functions[0].Parameters = append(m.Functions[0].Parameters, p)
			edge.Arguments[1] = 200
		}},
		{"swapped normalized tuple", "same normalized range start/length tuple", OpSliceReborrowShared, func(_ *Module, _ *Block, edge *BranchTarget) {
			edge.Arguments[1], edge.Arguments[2] = edge.Arguments[2], edge.Arguments[1]
		}},
		{"equal constant is a different SSA root", "same normalized range start/length tuple", OpSliceReborrowShared, func(m *Module, _ *Block, edge *BranchTarget) {
			for _, block := range m.Functions[0].Blocks {
				last := len(block.Operations) - 1
				if len(block.Operations[last].Successors) == 0 || &block.Operations[last].Successors[0] != edge {
					continue
				}
				typ := m.Functions[0].ReturnType
				constant := Operation{Kind: OpConstInt, Integer: big.NewInt(0), Results: []Value{{ID: 200, Type: typ, Ownership: OwnershipImmediate}}}
				block.Operations = append(append(append([]Operation(nil), block.Operations[:last]...), constant), block.Operations[last])
				block.Operations[len(block.Operations)-1].Successors[0].Arguments[1] = 200
				break
			}
		}},
		{"missing temporal guard", "requires its ref.is-valid guard", OpSliceElementPlace, func(_ *Module, block *Block, _ *BranchTarget) { block.Operations[0].ReferenceGuard = 0 }},
		{"guard bypass", "bounds guard true edge", OpSliceElementPlace, func(m *Module, block *Block, edge *BranchTarget) {
			fn := m.Functions[0]
			boolean := m.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
			fn.Parameters = append(fn.Parameters, Parameter{Name: "choose", Value: Value{ID: 200, Type: boolean, Ownership: OwnershipImmediate}})
			old := fn.Entry
			fn.Entry = 200
			args := append([]ValueID(nil), edge.Arguments...)
			fn.Blocks = append(fn.Blocks, &Block{ID: 200, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{200}, Successors: []BranchTarget{{Block: old}, {Block: block.ID, Arguments: args}}}}})
		}},
	} {
		for _, width := range []uint16{32, 64} {
			t.Run(fmt.Sprintf("%d/%s", width, tc.name), func(t *testing.T) {
				m := sliceIndexModule(false, true, true, "read")
				if tc.kind == OpSliceReborrowShared {
					m = sliceBorrowModule(true, false, true)
				}
				m.PointerWidthBits = width
				block, edge := forwardSliceProofOperation(t, m, tc.kind)
				if err := Verify(m); err != nil {
					t.Fatal("forwarded baseline", err)
				}
				tc.change(m, block, edge)
				if err := Verify(m); err == nil || !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("want %s: %v", tc.reason, err)
				}
			})
		}
	}
}

// The absent proof marker cannot become a guard through a malicious alias
// map; reference guards and bounds guards both reserve zero for absence.
// Rules: rules/compiler/semantic_ir.md — §126.14–18,23.
func TestSliceCFGZeroGuardStaysAbsent(t *testing.T) {
	if forwardedValue(0, map[ValueID]ValueID{0: 2}) != 0 {
		t.Fatal("absent guard acquired proof")
	}
}

// Slice proof forwarding cannot manufacture P15 direct-reference facts. In
// particular, normalizing a forwarded dereference source would skip the
// original P15 source/validity boundary on a reference-derived array Place.
// Rules: rules/compiler/semantic_ir.md — §§66,126.7–8,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§84–86,116.
func TestSliceCFGPreservesP15ReferenceBoundary(t *testing.T) {
	m := sliceBorrowModule(false, true, false)
	fn := m.Functions[0]
	array := fn.Storages[0].Type
	fn.Storages = nil
	fn.Parameters[0].Value.Type = m.Types.Intern(Type{Kind: TypeReference, Element: array, Mutable: true})
	fn.Parameters[0].Value.Ownership = OwnershipMutableReference
	fn.Parameters[0].Reference = p15Proven(1, 0)
	ops := fn.Blocks[0].Operations[2:]
	ops[0].Kind, ops[0].Storage, ops[0].Operands = OpReferenceDerefPlace, 0, []ValueID{10}
	ops[0].Place = nil
	ops[0].Reference = cloneReferenceMetadata(fn.Parameters[0].Reference)
	ops[3].Reference = cloneReferenceMetadata(fn.Parameters[0].Reference)
	ops[3].Reference.Borrow, ops[3].Reference.Lifetime, ops[3].Reference.SourceBorrow = 2, 2, 1
	fn.Blocks[0].Operations = ops
	if err := Verify(m); err != nil {
		t.Fatal("P15 source baseline", err)
	}
	parameter := fn.Parameters[0].Value
	parameter.ID = 50
	fn.Blocks[0].Parameters = []Value{parameter}
	fn.Blocks[0].Operations[0].Operands = []ValueID{50}
	fn.Entry = 0
	fn.Blocks = append([]*Block{{ID: 0, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{10}}}}}}}, fn.Blocks...)
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "distinct borrow derived from its source") {
		t.Fatal("slice view invented P15 source facts", err)
	}
}

// Move one proof-consuming operation and its continuation into a fresh block,
// forwarding precisely its original operand values through typed arguments.
// This is a white-box IR transformation, not a new source producer.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.23.
func forwardSliceProofOperation(t *testing.T, m *Module, kind OpKind) (*Block, *BranchTarget) {
	t.Helper()
	fn := m.Functions[0]
	values := map[ValueID]Value{}
	maxValue, maxBlock := ValueID(0), BlockID(0)
	add := func(v Value) {
		values[v.ID] = v
		if v.ID > maxValue {
			maxValue = v.ID
		}
	}
	for _, p := range fn.Parameters {
		add(p.Value)
	}
	for _, block := range fn.Blocks {
		if block.ID > maxBlock {
			maxBlock = block.ID
		}
		for _, p := range block.Parameters {
			add(p)
		}
		for _, op := range block.Operations {
			for _, v := range op.Results {
				add(v)
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != kind {
				continue
			}
			tail := append([]Operation(nil), block.Operations[index:]...)
			tail[0].Operands = make([]ValueID, len(op.Operands))
			parameters := make([]Value, len(op.Operands))
			for i, id := range op.Operands {
				parameter := values[id]
				maxValue++
				parameter.ID = maxValue
				parameters[i] = parameter
				tail[0].Operands[i] = parameter.ID
			}
			next := &Block{ID: maxBlock + 1, Parameters: parameters, Operations: tail}
			branch := Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: next.ID, Arguments: append([]ValueID(nil), op.Operands...)}}}
			block.Operations = append(append([]Operation(nil), block.Operations[:index]...), branch)
			fn.Blocks = append(fn.Blocks, next)
			return next, &block.Operations[len(block.Operations)-1].Successors[0]
		}
	}
	t.Fatal("fixture lost operation", kind)
	return nil, nil
}
