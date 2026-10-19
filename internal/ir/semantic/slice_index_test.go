package semantic

import (
	"math/big"
	"strings"
	"testing"
)

// Indexing forms a non-owning Place, retaining original wide index SSA and
// using P15 reads/writes/borrows without transferring a backing element.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,81–82.
func TestSliceIndexOperations(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				for _, runtime := range []bool{false, true} {
					uses := []string{"read", "shared-borrow"}
					if mutable {
						uses = append(uses, "write", "mutable-borrow")
					}
					for _, use := range uses {
						module := sliceIndexModule(mutable, dynamic, runtime, use)
						module.PointerWidthBits = width
						if err := Verify(module); err != nil {
							t.Fatalf("%d mutable=%v dynamic=%v runtime=%v %s: %v\n%s", width, mutable, dynamic, runtime, use, err, Format(module))
						}
						text := Format(module)
						if !strings.Contains(text, "slice.element-place %1, %10") || text != Format(module) {
							t.Fatal("missing/non-deterministic Place printing")
						}
						for _, forbidden := range []string{"array.extract", "array.replace", "storage.load", "slice.move-mut", "slice.copy-shared"} {
							if strings.Contains(text, forbidden) {
								t.Fatalf("indexing introduced %s", forbidden)
							}
						}
						if !dynamic && !runtime {
							module.Functions[0].NoPanic = true
							module.Functions[0].NoPanicSource = "written"
							if err := Verify(module); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
		}
	}
}

// Bounds comparisons consume the original signed/unsigned carrier, including
// indexes wider than the target; no uint cast precedes validation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§52,81–82.
func TestSliceIndexOriginalIntegerCarriers(t *testing.T) {
	for _, signed := range []bool{false, true} {
		for _, bits := range []uint16{8, 128, 256} {
			m := sliceIndexModule(false, false, true, "read")
			kind, name := TypeUint, "wide-unsigned"
			if signed {
				kind, name = TypeInt, "wide-signed"
			}
			id := m.Types.Intern(Type{Kind: kind, Name: name, BitWidth: bits, Signed: signed})
			m.Functions[0].Parameters[1].Value.Type = id
			m.Functions[0].Blocks[0].Operations[0].ArrayIndexSigned = signed
			m.Functions[0].Blocks[1].Operations[0].ArrayIndexSigned = signed
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// The spatial failure edge can materialize canonical IndexError and propagate
// Result, independently of the unrecoverable stale-reference endpoint.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§53,58,81–83.
func TestSliceIndexFallibleFailure(t *testing.T) {
	m := sliceIndexModule(false, true, true, "read")
	fn := m.Functions[0]
	uintType := m.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	errorType := m.Types.Intern(Type{Kind: TypeEnum, Name: "IndexError", Module: "core", Identity: "core::IndexError", Underlying: uintType})
	m.Enums = []EnumDefinition{{TypeID: errorType, SymbolID: "core::IndexError", Name: "IndexError", Underlying: uintType, RepresentationKind: EnumRepresentationInteger, Cases: []EnumCase{{ID: 0, Name: "OutOfBounds", Value: big.NewInt(0)}}}}
	result := m.Types.Intern(Type{Kind: TypeResult, Name: "Result[int32,IndexError]", Success: fn.ReturnType, Error: errorType})
	fn.ReturnType = result
	for _, block := range fn.Blocks {
		if block.ID == 4 {
			block.Operations = append(block.Operations[:len(block.Operations)-1], Operation{Kind: OpResultOk, Operands: []ValueID{6}, Results: []Value{{ID: 20, Type: result, Ownership: OwnershipImmediate}}}, Operation{Kind: OpReturn, Operands: []ValueID{20}})
		}
		if block.ID == 5 {
			block.Operations = []Operation{
				{Kind: OpEnumConstant, EnumCase: 0, Results: []Value{{ID: 21, Type: errorType, Ownership: OwnershipImmediate}}},
				{Kind: OpResultErr, Operands: []ValueID{21}, Results: []Value{{ID: 22, Type: result, Ownership: OwnershipImmediate}}},
				{Kind: OpReturn, Operands: []ValueID{22}},
			}
		}
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	fn.NoPanic, fn.NoPanicSource = true, "written"
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "InvalidReferenceGeneration") {
		t.Fatalf("try swallowed temporal failure: %v", err)
	}
}

// Malformed source/index pairs, widening, missing proofs, wrong guard edges
// and temporal/spatial substitution cannot authorize an element Place.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§52–58,81–84.
func TestSliceIndexRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name             string
		dynamic, runtime bool
		change           func(*Module)
	}{
		{"wrong signedness", false, true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].ArrayIndexSigned = false }},
		{"wrong predicate result", false, true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Results[0].Type = 1 }},
		{"missing proof", false, false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].ArrayProofKind = "" }},
		{"negative proven index", false, false, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Integer = big.NewInt(-1) }},
		{"missing bounds guard", false, true, func(m *Module) { m.Functions[0].Blocks[1].Operations[0].ArrayGuard = 0 }},
		{"different index SSA", false, true, func(m *Module) {
			fn := m.Functions[0]
			p := fn.Parameters[1]
			p.Value.ID = 11
			p.Name = "other"
			fn.Parameters = append(fn.Parameters, p)
			fn.Blocks[1].Operations[0].Operands[1] = 11
		}},
		{"different slice SSA", false, true, func(m *Module) {
			fn := m.Functions[0]
			p := fn.Parameters[0]
			p.Value.ID = 11
			p.Name = "other"
			fn.Parameters = append(fn.Parameters, p)
			fn.Blocks[1].Operations[0].Operands[0] = 11
		}},
		{"authority widening", false, true, func(m *Module) {
			op := &m.Functions[0].Blocks[1].Operations[0]
			op.Results[0].Type = m.Types.Intern(Type{Kind: TypePlace, Element: 1, Mutable: true})
		}},
		{"wrong element", false, true, func(m *Module) {
			op := &m.Functions[0].Blocks[1].Operations[0]
			op.Results[0].Type = m.Types.Intern(Type{Kind: TypePlace, Element: m.Functions[0].Parameters[1].Value.Type})
		}},
		{"changed source lifetime", false, true, func(m *Module) { m.Functions[0].Blocks[1].Operations[0].Reference.Lifetime++ }},
		{"changed source storage", false, true, func(m *Module) { m.Functions[0].Blocks[1].Operations[0].Reference.OriginStorages[0]++ }},
		{"reversed bounds edges", false, true, func(m *Module) {
			op := &m.Functions[0].Blocks[0].Operations[1]
			op.Successors[0], op.Successors[1] = op.Successors[1], op.Successors[0]
		}},
		{"array failure provenance", false, true, func(m *Module) { m.Functions[0].Blocks[2].Operations[0].ArrayOperation = "fixed-array-index" }},
		{"predicate not immediately branched", false, true, func(m *Module) {
			b := m.Functions[0].Blocks[0]
			b.Operations = append(b.Operations[:1], append([]Operation{{Kind: OpConstInt, Integer: big.NewInt(1), Results: []Value{{ID: 30, Type: 1, Ownership: OwnershipImmediate}}}}, b.Operations[1:]...)...)
		}},
		{"bounds guard bypass", false, true, func(m *Module) {
			fn := m.Functions[0]
			boolean := fn.Blocks[0].Operations[0].Results[0].Type
			fn.Parameters = append(fn.Parameters, Parameter{Name: "choose", Value: Value{ID: 30, Type: boolean, Ownership: OwnershipImmediate}})
			fn.Entry = 6
			fn.Blocks = append(fn.Blocks, &Block{ID: 6, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{30}, Successors: []BranchTarget{{Block: 1}, {Block: 4}}}}})
		}},
		{"missing temporal predicate guard", true, true, func(m *Module) { m.Functions[0].Blocks[1].Operations[0].ReferenceGuard = 0 }},
		{"missing temporal place guard", true, true, func(m *Module) { m.Functions[0].Blocks[2].Operations[0].ReferenceGuard = 0 }},
		{"wrong epoch", true, true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.EpochDependency++ }},
		{"bounds instead of stale", true, true, func(m *Module) {
			m.Functions[0].Blocks[4].Operations[0] = Operation{Kind: OpBoundsFailure, ArrayOperation: "slice-index"}
		}},
		{"spatial predicate before validity", true, true, func(m *Module) {
			fn := m.Functions[0]
			pred := fn.Blocks[1].Operations[0]
			fn.Blocks[0].Operations = append([]Operation{pred}, fn.Blocks[0].Operations...)
			fn.Blocks[1].Operations = fn.Blocks[1].Operations[1:]
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sliceIndexModule(false, tt.dynamic, tt.runtime, "read")
			tt.change(m)
			if err := Verify(m); err == nil {
				t.Fatal("accepted malformed slice indexing")
			}
		})
	}
}

// The nontrivial element-transfer boundary stays with P17 even though P15
// Place reads/writes are available; shared slices never authorize writes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§55–57.
func TestSliceIndexRejectsSharedMutationAndNontrivialRead(t *testing.T) {
	for _, use := range []string{"write", "mutable-borrow"} {
		if err := Verify(sliceIndexModule(false, false, false, use)); err == nil {
			t.Fatalf("accepted shared %s", use)
		}
	}
	m := sliceIndexModule(true, false, false, "read")
	ref := m.Types.Intern(Type{Kind: TypeReference, Element: 1, Mutable: true})
	view, _ := m.Types.InternSlice(ref, true)
	fn := m.Functions[0]
	fn.Parameters[0].Value.Type = view
	fn.ReturnType = ref
	fn.Blocks[0].Operations[0].Results[0].Type = m.Types.Intern(Type{Kind: TypePlace, Element: ref, Mutable: true})
	fn.Blocks[0].Operations[1].Results[0].Type = ref
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "copy-trivial") {
		t.Fatalf("element move escaped boundary: %v", err)
	}
}

// Assemble an original int128 index, optional temporal gate, optional bounds
// gate and one existing P15 element use; source-to-IR production is P16-35.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,81–82.
func sliceIndexModule(mutable, dynamic, runtime bool, use string) *Module {
	m := sliceLengthModule(mutable, dynamic)
	fn := m.Functions[0]
	fn.ID, fn.Name = "main::Index", "Index"
	fn.ReturnType = 1
	index := m.Types.Intern(Type{Kind: TypeInt, Name: "int128", BitWidth: 128, Signed: true})
	boolean := m.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	fn.Parameters = append(fn.Parameters, Parameter{Name: "index", Value: Value{ID: 10, Type: index, Ownership: OwnershipImmediate}})
	place := m.Types.Intern(Type{Kind: TypePlace, Element: 1, Mutable: mutable})
	element := Operation{Kind: OpSliceElementPlace, Operands: []ValueID{1, 10}, Results: []Value{{ID: 5, Type: place, Ownership: OwnershipCompilerTemporary}}, ArrayIndexSigned: true, ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofAnalysis, Reference: cloneReferenceMetadata(fn.Parameters[0].Reference)}
	predicate := Operation{Kind: OpSliceIndexInBounds, Operands: []ValueID{1, 10}, Results: []Value{{ID: 4, Type: boolean, Ownership: OwnershipImmediate}}, ArrayIndexSigned: true}
	if dynamic {
		element.ReferenceGuard = 2
		predicate.ReferenceGuard = 2
	}
	if runtime {
		element.ArrayCheckKind = ArrayIndexRuntimeCheck
		element.ArrayProofKind = ArrayIndexProofGuarded
		element.ArrayGuard = 4
	}
	ops := []Operation{element}
	switch use {
	case "read":
		ops = append(ops, Operation{Kind: OpPlaceRead, Operands: []ValueID{5}, Results: []Value{{ID: 6, Type: 1, Ownership: OwnershipImmediate}}})
	case "write":
		ops = append(ops, Operation{Kind: OpConstInt, Integer: big.NewInt(42), Results: []Value{{ID: 6, Type: 1, Ownership: OwnershipImmediate}}}, Operation{Kind: OpPlaceWrite, Operands: []ValueID{5, 6}})
	default:
		mut := use == "mutable-borrow"
		kind, ownership := OpReferenceBorrowShared, OwnershipSharedReference
		if mut {
			kind, ownership = OpReferenceBorrowMutable, OwnershipMutableReference
		}
		ref := m.Types.Intern(Type{Kind: TypeReference, Element: 1, Mutable: mut})
		facts := cloneReferenceMetadata(fn.Parameters[0].Reference)
		facts.Borrow = 3
		facts.SourceBorrow = 0
		ops = append(ops, Operation{Kind: kind, Operands: []ValueID{5}, Results: []Value{{ID: 6, Type: ref, Ownership: ownership}}, Reference: facts}, Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{6}, Reference: cloneReferenceMetadata(facts)}, Operation{Kind: OpConstInt, Integer: big.NewInt(0), Results: []Value{{ID: 7, Type: 1, Ownership: OwnershipImmediate}}})
	}
	result := ValueID(6)
	if strings.Contains(use, "borrow") {
		result = 7
	}
	ops = append(ops, Operation{Kind: OpReturn, Operands: []ValueID{result}})
	workID := BlockID(1)
	if dynamic {
		workID = 2
	}
	var blocks []*Block
	if dynamic {
		blocks = append(blocks, fn.Blocks[0])
	}
	if runtime {
		blocks = append(blocks, &Block{ID: workID, Operations: []Operation{predicate, {Kind: OpCondBranch, Operands: []ValueID{4}, Successors: []BranchTarget{{Block: 4}, {Block: 5}}}}}, &Block{ID: 4, Operations: ops}, &Block{ID: 5, Operations: []Operation{{Kind: OpBoundsFailure, ArrayOperation: "slice-index"}}})
	} else {
		blocks = append(blocks, &Block{ID: workID, Operations: ops})
	}
	if dynamic {
		blocks = append(blocks, fn.Blocks[2])
	}
	fn.Blocks = blocks
	return m
}

// Previously formed element Places are still access paths through the ended
// or moved slice holder; retaining Place SSA cannot extend access lifetime.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§41,50,63.
func TestSliceIndexPlaceCannotOutliveHolder(t *testing.T) {
	for _, move := range []bool{false, true} {
		m := sliceIndexModule(move, false, false, "read")
		fn := m.Functions[0]
		boundary := Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{1}, Reference: cloneReferenceMetadata(fn.Parameters[0].Reference)}
		if move {
			boundary.Kind = OpSliceMoveMutable
			boundary.Results = []Value{{ID: 30, Type: fn.Parameters[0].Value.Type, Ownership: OwnershipMutableReference}}
			boundary.Reference.SourceBorrow = boundary.Reference.Borrow
		}
		b := fn.Blocks[0]
		b.Operations = append(b.Operations[:1], append([]Operation{boundary}, b.Operations[1:]...)...)
		if err := Verify(m); err == nil || !strings.Contains(err.Error(), "used after") {
			t.Fatalf("derived Place escaped holder boundary: %v", err)
		}
	}
}

// The runtime predicate retains exact constants outside target uint, including
// the signed minimum; classification is performed before physical narrowing.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§52,81.
func TestSliceIndexRetainsWideConstantOperands(t *testing.T) {
	for _, signed := range []bool{false, true} {
		m := sliceIndexModule(false, false, true, "read")
		fn := m.Functions[0]
		kind, name, bits := TypeUint, "uint256", uint16(256)
		value := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		if signed {
			kind, name, bits = TypeInt, "int128", 128
			value.Neg(new(big.Int).Lsh(big.NewInt(1), 127))
		}
		index := m.Types.Intern(Type{Kind: kind, Name: name, BitWidth: bits, Signed: signed})
		fn.Parameters = fn.Parameters[:1]
		b := fn.Blocks[0]
		b.Operations = append([]Operation{{Kind: OpConstInt, Integer: value, Results: []Value{{ID: 10, Type: index, Ownership: OwnershipImmediate}}}}, b.Operations...)
		b.Operations[1].ArrayIndexSigned = signed
		fn.Blocks[1].Operations[0].ArrayIndexSigned = signed
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		if !strings.Contains(text, value.String()) || strings.Contains(text, "numeric.convert") {
			t.Fatal("lost original wide endpoint")
		}
	}
}
