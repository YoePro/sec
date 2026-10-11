package semantic

import (
	"math/big"
	"strings"
	"testing"
)

// package15Types interns the shared types of the P15 Place/reference tests.
type package15Types struct {
	table                                  *TypeTable
	intType, boolType, pairType, unionType TypeID
	arrayType, wideArrayType               TypeID
	placeIntRO, placeIntRW, placePairRW    TypeID
	placeArrayRO, placeWideRO, placeUnion  TypeID
	refInt, refMutInt                      TypeID
}

func newPackage15Types() package15Types {
	types := NewTypeTable()
	t := package15Types{table: types}
	t.intType = types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	t.boolType = types.Intern(Type{Kind: TypeBool, Name: "bool"})
	t.pairType = types.Intern(Type{Kind: TypeStruct, Name: "Pair", Module: "main", Identity: "main::Pair"})
	t.unionType = types.Intern(Type{Kind: TypeUnion, Name: "Value", Module: "main", Identity: "main::Value"})
	t.arrayType = types.Intern(Type{Kind: TypeArray, Name: "int[3]", Element: t.intType, Length: "3"})
	t.wideArrayType = types.Intern(Type{Kind: TypeArray, Name: "int[huge]", Element: t.intType, Length: "18446744073709551620"})
	t.placeIntRO = types.Intern(Type{Kind: TypePlace, Element: t.intType})
	t.placeIntRW = types.Intern(Type{Kind: TypePlace, Element: t.intType, Mutable: true})
	t.placePairRW = types.Intern(Type{Kind: TypePlace, Element: t.pairType, Mutable: true})
	t.placeArrayRO = types.Intern(Type{Kind: TypePlace, Element: t.arrayType})
	t.placeWideRO = types.Intern(Type{Kind: TypePlace, Element: t.wideArrayType})
	t.placeUnion = types.Intern(Type{Kind: TypePlace, Element: t.unionType})
	t.refInt = types.Intern(Type{Kind: TypeReference, Element: t.intType})
	t.refMutInt = types.Intern(Type{Kind: TypeReference, Element: t.intType, Mutable: true})
	return t
}

func p15Place(root uint32, kind PlaceRootKind) *PlaceMetadata {
	return &PlaceMetadata{Root: root, RootKind: kind, Storage: root, AddressSpace: AddressSpaceOrdinary}
}

func p15Proven(borrow, source uint32) *ReferenceMetadata {
	return &ReferenceMetadata{Borrow: borrow, Lifetime: borrow, SourceBorrow: source, Validity: ReferenceProven, ProofProvenance: "lexical-borrow",
		AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "local", OriginStorages: []uint32{1}}
}

func p15Dynamic(borrow uint32) *ReferenceMetadata {
	return &ReferenceMetadata{Borrow: borrow, Lifetime: borrow, Validity: ReferenceDynamicEpoch, InvalidationDomain: 1, EpochDependency: 1,
		AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
}

// package15Module builds one function per area: storage borrows, reads and
// writes through ref mut, shared reborrow/copy/compare and borrow end,
// constant and wide fixed-array element Places, a struct field Place, a
// variant-proven union payload Place, and a dynamic-epoch reference guarded
// by its validity predicate with the invalid-reference-generation endpoint.
func package15Module() *Module {
	t := newPackage15Types()
	v := func(id ValueID, typ TypeID, ownership OwnershipClass) Value {
		return Value{ID: id, Type: typ, Ownership: ownership}
	}
	imm := OwnershipImmediate
	tmp := OwnershipCompilerTemporary

	borrow := &Function{ID: "main::Borrow()", Name: "Borrow", ReturnType: t.intType, Entry: 0,
		Storages: []Storage{{ID: 1, Name: "value", Type: t.intType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
	borrow.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpConstInt, Integer: big.NewInt(1), Results: []Value{v(1, t.intType, imm)}},
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{1}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(2, t.placeIntRW, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpReferenceBorrowMutable, Operands: []ValueID{2}, Results: []Value{v(3, t.refMutInt, OwnershipMutableReference)}, Reference: p15Proven(1, 0)},
		{Kind: OpReferenceDerefPlace, Operands: []ValueID{3}, Results: []Value{v(4, t.placeIntRW, tmp)}, Reference: p15Proven(1, 0)},
		{Kind: OpConstInt, Integer: big.NewInt(2), Results: []Value{v(5, t.intType, imm)}},
		{Kind: OpPlaceWrite, Operands: []ValueID{4, 5}},
		{Kind: OpPlaceRead, Operands: []ValueID{4}, Results: []Value{v(6, t.intType, imm)}},
		{Kind: OpReferenceReborrowShared, Operands: []ValueID{3}, Results: []Value{v(7, t.refInt, OwnershipSharedReference)}, Reference: p15Proven(2, 1)},
		{Kind: OpReferenceCopyShared, Operands: []ValueID{7}, Results: []Value{v(8, t.refInt, OwnershipSharedReference)}, Reference: p15Proven(2, 2)},
		{Kind: OpReferenceCompare, Operator: "eq", Operands: []ValueID{7, 8}, Results: []Value{v(9, t.boolType, imm)}},
		{Kind: OpReferenceEndBorrow, Operands: []ValueID{8}},
		{Kind: OpReferenceEndBorrow, Operands: []ValueID{7}},
		{Kind: OpReferenceMove, Operands: []ValueID{3}, Results: []Value{v(10, t.refMutInt, OwnershipMutableReference)}, Reference: p15Proven(1, 1)},
		{Kind: OpReferenceEndBorrow, Operands: []ValueID{10}},
		{Kind: OpReturn, Operands: []ValueID{6}},
	}}}

	elements := &Function{ID: "main::Elements(int[3],int[huge],int)", Name: "Elements", ReturnType: t.intType, Entry: 0,
		Parameters: []Parameter{{Name: "values", Value: v(1, t.arrayType, imm)}, {Name: "wide", Value: v(2, t.wideArrayType, imm)}, {Name: "index", Value: v(3, t.intType, imm)}}}
	wideIndex, _ := new(big.Int).SetString("18446744073709551619", 10)
	elements.Blocks = []*Block{
		{ID: 0, Operations: []Operation{
			{Kind: OpPlaceValueRoot, Operands: []ValueID{1}, Results: []Value{v(4, t.placeArrayRO, tmp)}, Place: p15Place(1, PlaceRootParameter)},
			{Kind: OpConstInt, Integer: big.NewInt(1), Results: []Value{v(5, t.intType, imm)}},
			{Kind: OpPlaceArrayElement, Operands: []ValueID{4, 5}, Integer: big.NewInt(1), ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofConstant, ArrayIndexSigned: true,
				Results: []Value{v(6, t.placeIntRO, tmp)}, Place: p15Place(1, PlaceRootParameter)},
			{Kind: OpPlaceRead, Operands: []ValueID{6}, Results: []Value{v(7, t.intType, imm)}},
			{Kind: OpPlaceValueRoot, Operands: []ValueID{2}, Results: []Value{v(8, t.placeWideRO, tmp)}, Place: p15Place(2, PlaceRootParameter)},
			{Kind: OpConstInt, Integer: wideIndex, Results: []Value{v(9, t.intType, imm)}},
			{Kind: OpPlaceArrayElement, Operands: []ValueID{8, 9}, Integer: wideIndex, ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofConstant, ArrayIndexSigned: true,
				Results: []Value{v(10, t.placeIntRO, tmp)}, Place: p15Place(2, PlaceRootParameter)},
			{Kind: OpPlaceArrayIndexInBounds, Operands: []ValueID{4, 3}, Results: []Value{v(11, t.boolType, imm)}},
			{Kind: OpCondBranch, Operands: []ValueID{11}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}},
		}},
		{ID: 1, Operations: []Operation{
			{Kind: OpPlaceArrayElement, Operands: []ValueID{4, 3}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayIndexSigned: true, ArrayGuard: 11,
				Results: []Value{v(12, t.placeIntRO, tmp)}, Place: &PlaceMetadata{Root: 1, RootKind: PlaceRootParameter, Storage: 1, AddressSpace: AddressSpaceOrdinary, DynamicIndexID: 1}},
			{Kind: OpPlaceRead, Operands: []ValueID{12}, Results: []Value{v(13, t.intType, imm)}},
			{Kind: OpReturn, Operands: []ValueID{13}},
		}},
		{ID: 2, Operations: []Operation{{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index"}}},
	}

	payload := &Function{ID: "main::Payload(Value)", Name: "Payload", ReturnType: t.intType, Entry: 0,
		Parameters: []Parameter{{Name: "value", Value: v(1, t.unionType, imm)}}}
	payload.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpPlaceValueRoot, Operands: []ValueID{1}, Results: []Value{v(2, t.placeUnion, tmp)}, Place: p15Place(1, PlaceRootParameter)},
		{Kind: OpPlaceUnionPayload, Operands: []ValueID{2}, UnionVariant: 1, Results: []Value{v(3, t.placeIntRO, tmp)},
			Place: &PlaceMetadata{Root: 1, RootKind: PlaceRootParameter, Storage: 1, AddressSpace: AddressSpaceOrdinary, VariantProven: true}},
		{Kind: OpPlaceRead, Operands: []ValueID{3}, Results: []Value{v(4, t.intType, imm)}},
		{Kind: OpReturn, Operands: []ValueID{4}},
	}}}

	field := &Function{ID: "main::Field()", Name: "Field", ReturnType: t.intType, Entry: 0,
		Storages: []Storage{{ID: 1, Name: "pair", Type: t.pairType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
	field.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpConstInt, Integer: big.NewInt(1), Results: []Value{v(1, t.intType, imm)}},
		{Kind: OpStructConstruct, Operands: []ValueID{1, 1}, Results: []Value{v(2, t.pairType, imm)},
			StructOrigins: []StructFieldOrigin{StructOriginExplicit, StructOriginExplicit}, StructActions: []StructFieldAction{StructActionCopyTrivial, StructActionCopyTrivial}},
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{2}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(3, t.placePairRW, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpPlaceField, Operands: []ValueID{3}, StructField: 1, Results: []Value{v(4, t.placeIntRW, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpReferenceBorrowShared, Operands: []ValueID{4}, Results: []Value{v(5, t.refInt, OwnershipSharedReference)}, Reference: p15Proven(1, 0)},
		{Kind: OpReferenceDerefPlace, Operands: []ValueID{5}, Results: []Value{v(6, t.placeIntRO, tmp)}, Reference: p15Proven(1, 0)},
		{Kind: OpPlaceRead, Operands: []ValueID{6}, Results: []Value{v(7, t.intType, imm)}},
		{Kind: OpReferenceEndBorrow, Operands: []ValueID{5}},
		{Kind: OpReturn, Operands: []ValueID{7}},
	}}}

	dynamic := &Function{ID: "main::Dynamic()", Name: "Dynamic", ReturnType: t.intType, Entry: 0,
		Storages: []Storage{{ID: 1, Name: "slot", Type: t.intType, Class: StorageLocalAutomatic, AddressTaken: true}}}
	dynamic.Blocks = []*Block{
		{ID: 0, Operations: []Operation{
			{Kind: OpConstInt, Integer: big.NewInt(4), Results: []Value{v(1, t.intType, imm)}},
			{Kind: OpStorageDeclare, Storage: 1},
			{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{1}},
			{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(2, t.placeIntRO, tmp)}, Place: p15Place(1, PlaceRootLocal)},
			{Kind: OpReferenceBorrowShared, Operands: []ValueID{2}, Results: []Value{v(3, t.refInt, OwnershipSharedReference)}, Reference: p15Dynamic(1)},
			{Kind: OpReferenceIsValid, Operands: []ValueID{3}, Results: []Value{v(4, t.boolType, imm)}, Reference: p15Dynamic(1)},
			{Kind: OpCondBranch, Operands: []ValueID{4}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}},
		}},
		{ID: 1, Operations: []Operation{
			{Kind: OpReferenceDerefPlace, Operands: []ValueID{3}, ReferenceGuard: 4, Results: []Value{v(5, t.placeIntRO, tmp)}, Reference: p15Dynamic(1)},
			{Kind: OpPlaceRead, Operands: []ValueID{5}, Results: []Value{v(6, t.intType, imm)}},
			{Kind: OpReturn, Operands: []ValueID{6}},
		}},
		{ID: 2, Operations: []Operation{{Kind: OpReferenceGenerationFailure, PanicReasonID: invalidReferenceGenerationID, PanicReason: invalidReferenceGenerationName}}},
	}

	return &Module{Version: Version, Identity: "main", Types: t.table,
		Functions: []*Function{borrow, elements, payload, field, dynamic},
		Structs: []StructDefinition{{TypeID: t.pairType, SymbolID: "main::Pair", Name: "Pair",
			Fields:             []StructFieldDefinition{{ID: 0, Name: "left", Type: t.intType}, {ID: 1, Name: "right", Type: t.intType}},
			CopyClassification: "trivial", TriviallyDestructible: true, Defaultable: true}},
		Unions: []UnionDefinition{{TypeID: t.unionType, SymbolID: "main::Value", Name: "Value", CopyClassification: "trivial", TriviallyDestructible: true,
			Variants: []UnionVariantDefinition{{Index: 0, Name: "None", Kind: UnionVariantEmpty}, {Index: 1, Name: "One", Kind: UnionVariantSingle, Payload: t.intType}}}},
	}
}

// The P15 Place and direct-reference operations verify and print
// deterministically with stable identities, exact indexes and complete
// reference facts (P15-31..P15-38, P15-40).
func TestPackage15PlaceReferenceOperationsVerifyAndFormat(t *testing.T) {
	module := package15Module()
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if text != Format(package15Module()) {
		t.Fatal("printer output is not deterministic")
	}
	for _, want := range []string{
		"place<!1, rw>", "ref_mut<!1>", "ref<!1>",
		"place.storage-root $1 {root=#1 kind=local storage=#1 space=ordinary}",
		"ref.borrow-mut %2 {borrow=#1 lifetime=#1 validity=proven proof=lexical-borrow space=ordinary stability=Stable provenance=local origins=[#1]}",
		"ref.reborrow-shared %3 {borrow=#2 lifetime=#2 from=#1",
		"ref.compare %7, %8 eq",
		"place.array-element %8, %9 bounds=proven-safe proof=constant index=18446744073709551619",
		"index-id=#1",
		"place.union-payload %2 variant=#1",
		"variant-proven",
		"ref.deref-place %3 valid=%4",
		"domain=#1 epoch=#1",
		"fail.reference-generation reason=InvalidReferenceGeneration#8",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("formatted module lacks %q", want)
		}
	}
}

// Malformed Place/reference facts and every unsupported P15 ownership
// boundary are rejected without accepting partial IR (P15-34, P15-39, P15-40).
func TestPackage15VerifierRejectsMalformedPlaceReferenceIR(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Module, package15Types)
		want   string
	}{
		{"storage root wider than storage", func(m *Module, _ package15Types) {
			m.Functions[4].Blocks[0].Operations[3].Results[0].Type = typeOf(m, TypePlace, true)
		}, "authority exceeds storage mutability"},
		{"mutable borrow of read-only place", func(m *Module, t package15Types) {
			op := &m.Functions[4].Blocks[0].Operations[4]
			op.Kind = OpReferenceBorrowMutable
			op.Results[0].Type = t.refMutInt
		}, "ref.borrow-mut requires a read/write Place"},
		{"ref to ref mut reborrow", func(m *Module, t package15Types) {
			op := &m.Functions[0].Blocks[0].Operations[10]
			op.Kind = OpReferenceReborrowMutable
			op.Results[0].Type = t.refMutInt
		}, "cannot derive ref mut from ref"},
		{"copy of ref mut", func(m *Module, t package15Types) {
			op := &m.Functions[0].Blocks[0].Operations[10]
			op.Operands = []ValueID{3}
			op.Results[0].Type = t.refMutInt
		}, "ref mut is move-only"},
		{"reborrow keeps source borrow", func(m *Module, _ package15Types) { m.Functions[0].Blocks[0].Operations[9].Reference.Borrow = 1 }, "derives a new borrow"},
		{"uncheckable reference", func(m *Module, _ package15Types) {
			m.Functions[0].Blocks[0].Operations[4].Reference.Validity = ""
		}, "neither proven nor dynamically checkable"},
		{"proven reference without proof", func(m *Module, _ package15Types) {
			m.Functions[0].Blocks[0].Operations[4].Reference.ProofProvenance = ""
		}, "closed proof provenance"},
		{"write through read-only place", func(m *Module, t package15Types) {
			fn := m.Functions[3]
			fn.Blocks[0].Operations[7].Results[0].Type = t.placeIntRO
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:9], append([]Operation{{Kind: OpPlaceWrite, Operands: []ValueID{6, 1}}}, fn.Blocks[0].Operations[9:]...)...)
		}, "place.write requires a read/write Place"},
		{"field authority change", func(m *Module, t package15Types) {
			m.Functions[3].Blocks[0].Operations[5].Results[0].Type = t.placeIntRO
		}, "authority must be preserved"},
		{"constant index outside length", func(m *Module, _ package15Types) {
			m.Functions[1].Blocks[0].Operations[2].Integer = big.NewInt(3)
		}, "outside the exact length"},
		{"runtime element without guard", func(m *Module, _ package15Types) { m.Functions[1].Blocks[1].Operations[0].ArrayGuard = 0 }, "invalid runtime bounds provenance"},
		{"union payload without variant proof", func(m *Module, _ package15Types) { m.Functions[2].Blocks[0].Operations[1].Place.VariantProven = false }, "active-variant proof"},
		{"move-only read", func(m *Module, t package15Types) {
			m.Structs[0].CopyClassification = "move-only"
			m.Structs[0].TriviallyDestructible = false
			fn := m.Functions[3]
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:5], Operation{Kind: OpPlaceRead, Operands: []ValueID{3}, Results: []Value{{ID: 20, Type: t.pairType, Ownership: OwnershipImmediate}}}, Operation{Kind: OpReturn, Operands: []ValueID{1}})
		}, "copy-trivial referents only"},
		{"dynamic deref without guard", func(m *Module, _ package15Types) { m.Functions[4].Blocks[1].Operations[0].ReferenceGuard = 0 }, "requires its ref.is-valid guard"},
		{"dynamic guard without failure endpoint", func(m *Module, _ package15Types) {
			m.Functions[4].Blocks[2].Operations = []Operation{{Kind: OpReturn, Operands: []ValueID{1}}}
		}, "must terminate with fail.reference-generation"},
		{"failure endpoint with wrong reason", func(m *Module, _ package15Types) { m.Functions[4].Blocks[2].Operations[0].PanicReasonID = 1 }, "panic.invalid-reference-generation"},
		{"use after end borrow", func(m *Module, _ package15Types) {
			ops := m.Functions[0].Blocks[0].Operations
			ops[12], ops[11] = ops[11], ops[12]
		}, "used after its resolved borrow end"},
		{"place returned", func(m *Module, _ package15Types) { m.Functions[2].Blocks[0].Operations[3].Operands = []ValueID{3} }, "escapes through return"},
		{"place stored", func(m *Module, t package15Types) {
			m.Functions[0].Storages = append(m.Functions[0].Storages, Storage{ID: 2, Name: "escape", Type: t.placeIntRW, Mutable: true, Class: StorageLocalAutomatic})
			ops := m.Functions[0].Blocks[0].Operations
			m.Functions[0].Blocks[0].Operations = append(ops[:6:6], append([]Operation{{Kind: OpStorageDeclare, Storage: 2}, {Kind: OpStorageInit, Storage: 2, Operands: []ValueID{4}}}, ops[6:]...)...)
		}, "escapes through storage.init"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := package15Module()
			test.mutate(module, newPackage15TypesFrom(module))
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Verify error = %v, want %q", err, test.want)
			}
		})
	}
}

// newPackage15TypesFrom re-interns the shared types into the module's table,
// which returns the existing identities.
func newPackage15TypesFrom(module *Module) package15Types {
	types := module.Types
	t := package15Types{table: types}
	t.intType = types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	t.boolType = types.Intern(Type{Kind: TypeBool, Name: "bool"})
	t.pairType = types.Intern(Type{Kind: TypeStruct, Name: "Pair", Module: "main", Identity: "main::Pair"})
	t.placeIntRO = types.Intern(Type{Kind: TypePlace, Element: t.intType})
	t.placeIntRW = types.Intern(Type{Kind: TypePlace, Element: t.intType, Mutable: true})
	t.refInt = types.Intern(Type{Kind: TypeReference, Element: t.intType})
	t.refMutInt = types.Intern(Type{Kind: TypeReference, Element: t.intType, Mutable: true})
	return t
}

func typeOf(module *Module, kind TypeKind, mutable bool) TypeID {
	intType := module.Types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	return module.Types.Intern(Type{Kind: kind, Element: intType, Mutable: mutable})
}
