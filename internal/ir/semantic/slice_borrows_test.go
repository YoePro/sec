package semantic

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// Borrow/reborrow produces only a bounded reference, preserving authority,
// storage/epoch facts and a checked source-relative range on either target.
// Rules: rules/compiler/semantic_ir.md — §126.7–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46,78–80.
func TestSliceBorrowOperations(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, reborrow := range []bool{false, true} {
			for _, mutable := range []bool{false, true} {
				for _, dynamic := range []bool{false, true} {
					if dynamic && !reborrow {
						continue
					}
					t.Run(fmt.Sprintf("%d/reborrow=%v/mutable=%v/dynamic=%v", width, reborrow, mutable, dynamic), func(t *testing.T) {
						m := sliceBorrowModule(reborrow, mutable, dynamic)
						m.PointerWidthBits = width
						if err := Verify(m); err != nil {
							t.Fatal(err)
						}
						text := Format(m)
						if text != Format(m) || !strings.Contains(text, "borrow-") {
							t.Fatal(text)
						}
						for _, forbidden := range []string{"place.read", "storage.load", "array.extract", "array.replace", "slice.copy-shared"} {
							if strings.Contains(text, forbidden) {
								t.Fatal("construction copied backing elements", forbidden)
							}
						}
					})
				}
			}
		}
	}
	// Mutable storage can lend shared authority without mutating its type.
	m := sliceBorrowModule(true, true, false)
	op := &m.Functions[0].Blocks[0].Operations[2]
	op.Kind = OpSliceReborrowShared
	op.Results[0].Type, _ = m.Types.InternSlice(1, false)
	op.Results[0].Ownership = OwnershipSharedReference
	m.Functions[0].Blocks[0].Operations[3].Kind = OpSliceLength
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

// Malformed SSA tuples, range expansion, authority elevation, missing source
// validity and changed temporal/storage facts cannot mint a safe slice.
// Rules: rules/compiler/semantic_ir.md — §126.7–8,11;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46,83–86.
func TestSliceBorrowRejectsInvalidSources(t *testing.T) {
	tests := []struct {
		name              string
		reborrow, dynamic bool
		change            func(*Module, *Operation)
	}{
		{"writable source", false, false, func(m *Module, op *Operation) {
			p, _ := m.Types.Lookup(m.Functions[0].Blocks[0].Operations[2].Results[0].Type)
			p.Mutable = false
			m.Functions[0].Blocks[0].Operations[2].Results[0].Type = m.Types.Intern(p)
		}},
		{"shared to mutable", true, false, func(m *Module, op *Operation) {
			m.Functions[0].Parameters[0].Value.Type, _ = m.Types.InternSlice(1, false)
			m.Functions[0].Parameters[0].Value.Ownership = OwnershipSharedReference
		}},
		{"wrong element", true, false, func(m *Module, op *Operation) {
			elem := m.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
			op.Results[0].Type, _ = m.Types.InternSlice(elem, true)
		}},
		{"wrong ownership", true, false, func(m *Module, op *Operation) { op.Results[0].Ownership = OwnershipSharedReference }},
		{"swapped normalized results", true, false, func(m *Module, op *Operation) { op.Operands[1], op.Operands[2] = op.Operands[2], op.Operands[1] }},
		{"unchecked length", true, false, func(m *Module, op *Operation) { op.Operands[2] = 3 }},
		{"wrong fixed extent", false, false, func(m *Module, op *Operation) { m.Functions[0].Blocks[0].Operations[3].Integer = big.NewInt(9) }},
		{"invented source borrow", false, false, func(m *Module, op *Operation) { op.Reference.SourceBorrow = 99 }},
		{"changed Place storage", false, false, func(m *Module, op *Operation) { op.Reference.OriginStorages[0] = 99 }},
		{"missing facts", true, false, func(m *Module, op *Operation) { op.Reference = nil }},
		{"same borrow", true, false, func(m *Module, op *Operation) { op.Reference.Borrow = 1 }},
		{"wrong parent", true, false, func(m *Module, op *Operation) { op.Reference.SourceBorrow = 99 }},
		{"changed epoch", true, true, func(m *Module, op *Operation) { op.Reference.EpochDependency++ }},
		{"erased epoch", true, true, func(m *Module, op *Operation) {
			op.Reference.Validity, op.Reference.EpochDependency, op.Reference.ProofProvenance = ReferenceProven, 0, "invented"
		}},
		{"changed lifetime storage", true, false, func(m *Module, op *Operation) { op.Reference.OriginStorages[0]++ }},
		{"changed address space", true, false, func(m *Module, op *Operation) { op.Reference.AddressSpace = AddressSpaceMMIO }},
		{"changed pin", true, false, func(m *Module, op *Operation) { op.Reference.Pinned = true }},
		{"missing validity guard", true, true, func(m *Module, op *Operation) { op.ReferenceGuard = 0 }},
		{"different slice extent", true, false, func(m *Module, op *Operation) {
			p := m.Functions[0].Parameters[0]
			p.Name, p.Value.ID = "other", 99
			m.Functions[0].Parameters = append(m.Functions[0].Parameters, p)
			m.Functions[0].Blocks[0].Operations[0].Operands[0] = 99
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := sliceBorrowModule(test.reborrow, true, test.dynamic)
			block, index := 0, 5
			if test.reborrow {
				index = 2
			}
			if test.dynamic {
				block = 1
			}
			test.change(m, &m.Functions[0].Blocks[block].Operations[index])
			if err := Verify(m); err == nil {
				t.Fatal("accepted malformed slice construction")
			}
		})
	}
}

// A slice reborrow and copies of that child remain dependent on the parent;
// ending the parent must not turn a narrowed lifetime into an escaping view.
// Rules: rules/compiler/semantic_ir.md — §126.8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§43,63,86.
func TestSliceReborrowParentLifetime(t *testing.T) {
	for _, copied := range []bool{false, true} {
		m := sliceBorrowModule(true, false, false)
		fn := m.Functions[0]
		ops := fn.Blocks[0].Operations
		prefix := append([]Operation(nil), ops[:3]...)
		if copied {
			facts := cloneReferenceMetadata(ops[2].Reference)
			facts.SourceBorrow = facts.Borrow
			prefix = append(prefix, Operation{Kind: OpSliceCopyShared, Operands: []ValueID{6}, Results: []Value{{ID: 8, Type: ops[2].Results[0].Type, Ownership: OwnershipSharedReference}}, Reference: facts})
			ops[3].Operands[0] = 8
		}
		prefix = append(prefix, Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{1}, Reference: cloneReferenceMetadata(fn.Parameters[0].Reference)})
		fn.Blocks[0].Operations = append(prefix, ops[3:]...)
		if err := Verify(m); err == nil || !strings.Contains(err.Error(), "used after its resolved borrow end") {
			t.Fatalf("copied=%v: parent lifetime expanded: %v", copied, err)
		}
	}
}

// Checked normalization retains the success-only tuple. Construction on its
// failure edge cannot form a safe slice, even when source and types agree.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–43,78–80.
func TestSliceBorrowCheckedRange(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		m := sliceRangeModule(true)
		m.PointerWidthBits = width
		fn := m.Functions[0]
		uintType := fn.Parameters[0].Value.Type
		slice, err := m.Types.InternSlice(uintType, false)
		if err != nil {
			t.Fatal(err)
		}
		fn.Parameters[0].Value.Type = slice
		fn.Parameters[0].Value.Ownership = OwnershipSharedReference
		fn.Parameters[0].Reference = p15Proven(1, 0)
		rangeOp := fn.Blocks[0].Operations[0]
		rangeOp.Operands[0] = 10
		fn.Blocks[0].Operations = append([]Operation{{Kind: OpSliceLength, Operands: []ValueID{1}, Results: []Value{{ID: 10, Type: uintType, Ownership: OwnershipImmediate}}}, rangeOp}, fn.Blocks[0].Operations[1:]...)
		borrow := Operation{Kind: OpSliceReborrowShared, Operands: []ValueID{1, 4, 5}, Results: []Value{{ID: 11, Type: slice, Ownership: OwnershipSharedReference}}, Reference: p15Proven(2, 1)}
		fn.Blocks[2].Operations = append([]Operation{borrow}, fn.Blocks[2].Operations...)
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		fn.Blocks[2].Operations = fn.Blocks[2].Operations[1:]
		fn.Blocks[1].Operations = append([]Operation{borrow}, fn.Blocks[1].Operations...)
		if err := Verify(m); err == nil || !strings.Contains(err.Error(), "outside its guarded edge") {
			t.Fatalf("wrong-edge construction: %v", err)
		}
	}
}

// Empty and host-wide fixed extents remain exact; empty construction retains
// its source storage facts and does not require a dereferenceable element.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§8,25,29,42.
func TestSliceBorrowExactFixedExtents(t *testing.T) {
	for _, text := range []string{"0", "9223372036854775808"} {
		m := sliceBorrowModule(false, false, false)
		m.PointerWidthBits = 64
		fn := m.Functions[0]
		array, _ := m.Types.Lookup(fn.Storages[0].Type)
		array.Length = text
		arrayID := m.Types.Intern(array)
		fn.Storages[0].Type, fn.Parameters[0].Value.Type = arrayID, arrayID
		fn.Blocks[0].Operations[2].Results[0].Type = m.Types.Intern(Type{Kind: TypePlace, Element: arrayID})
		fn.Blocks[0].Operations[3].Integer, _ = new(big.Int).SetString(text, 10)
		if err := Verify(m); err != nil {
			t.Fatalf("extent=%s: %v", text, err)
		}
	}
}

// A fixed array reached through a safe reference keeps that reference's
// storage/lifetime dependency instead of pretending to borrow owned storage.
// Rules: rules/compiler/semantic_ir.md — §126.7–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46.
func TestSliceBorrowFromReferencedArray(t *testing.T) {
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
		t.Fatal(err)
	}
	end := Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{10}, Reference: cloneReferenceMetadata(fn.Parameters[0].Reference)}
	fn.Blocks[0].Operations = append(append([]Operation(nil), ops[:4]...), append([]Operation{end}, ops[4:]...)...)
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "used after its resolved borrow end") {
		t.Fatalf("array parent lifetime expanded: %v", err)
	}
}

// Build abstract fixed-array storage or a guarded slice parameter, normalize
// the full range, borrow it and observe its length without element access.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–49,78–80.
func sliceBorrowModule(reborrow, mutable, dynamic bool) *Module {
	m := sliceLengthModule(mutable, dynamic)
	fn := m.Functions[0]
	uintType := fn.ReturnType
	slice := fn.Parameters[0].Value.Type
	facts := cloneReferenceMetadata(fn.Parameters[0].Reference)
	facts.Borrow, facts.Lifetime, facts.SourceBorrow = 2, 2, 1
	kind := OpSliceReborrowShared
	ownership := OwnershipSharedReference
	if mutable {
		kind, ownership = OpSliceReborrowMutable, OwnershipMutableReference
	}
	normalize := Operation{Kind: OpSliceRangeNormalize, Operands: []ValueID{3}, Results: []Value{{ID: 4, Type: uintType, Ownership: OwnershipImmediate}, {ID: 5, Type: uintType, Ownership: OwnershipImmediate}}, SliceRange: &SliceRangeMetadata{ProofKind: SliceRangeProofOpenBound}}
	borrow := Operation{Kind: kind, Operands: []ValueID{1, 4, 5}, Results: []Value{{ID: 6, Type: slice, Ownership: ownership}}, Reference: facts}
	if dynamic {
		borrow.ReferenceGuard = 2
	}
	operations := []Operation{{Kind: OpSliceLength, Operands: []ValueID{1}, Results: []Value{{ID: 3, Type: uintType, Ownership: OwnershipImmediate}}, ReferenceGuard: borrow.ReferenceGuard}, normalize, borrow,
		{Kind: OpSliceLength, Operands: []ValueID{6}, Results: []Value{{ID: 7, Type: uintType, Ownership: OwnershipImmediate}}}, {Kind: OpReturn, Operands: []ValueID{7}}}
	if dynamic {
		// Only construction observes the parent dynamically. End the child
		// instead of adding a second observation without its own guard.
		operations = append(operations[:3], Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{6}, Reference: cloneReferenceMetadata(facts)}, Operation{Kind: OpReturn, Operands: []ValueID{5}})
		fn.Blocks[1].Operations = operations
	} else {
		fn.Blocks[0].Operations = operations
	}
	if !reborrow {
		array := m.Types.Intern(Type{Kind: TypeArray, Element: 1, Length: "8"})
		place := m.Types.Intern(Type{Kind: TypePlace, Element: array, Mutable: mutable})
		fn.Parameters = []Parameter{{Name: "initial", Value: Value{ID: 10, Type: array, Ownership: OwnershipOwned}}}
		fn.Storages = []Storage{{ID: 1, Name: "values", Type: array, Mutable: mutable, Class: StorageLocalAutomatic, AddressTaken: true}}
		facts.SourceBorrow = 0
		borrow.Kind = OpSliceBorrowShared
		if mutable {
			borrow.Kind = OpSliceBorrowMutable
		}
		fn.Blocks[0].Operations = append([]Operation{{Kind: OpStorageDeclare, Storage: 1}, {Kind: OpStorageInit, Storage: 1, Operands: []ValueID{10}}, {Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{{ID: 1, Type: place, Ownership: OwnershipCompilerTemporary}}, Place: &PlaceMetadata{Root: 1, RootKind: PlaceRootLocal, Storage: 1, AddressSpace: AddressSpaceOrdinary}}, {Kind: OpConstInt, Integer: big.NewInt(8), Results: []Value{{ID: 3, Type: uintType, Ownership: OwnershipImmediate}}}, normalize, borrow}, operations[3:]...)
	}
	return m
}
