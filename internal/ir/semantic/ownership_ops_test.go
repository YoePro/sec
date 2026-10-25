package semantic

import (
	"strings"
	"testing"
)

// ownershipModule exercises all ten own.* operations (package17 §§ 48–58):
// a move-only struct is moved into an address-taken local, moved out of its
// Place, which is then reinitialized (moved state, § 54), replaced and
// destroyed; a semantically copyable
// Text (custom free) is copied, discarded and destroyed; an int is trivially copied
// (also out of a Place) and a shared reference is discarded without
// destroying its referent.
func ownershipModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", BitWidth: 64})
	textType := types.Intern(Type{Kind: TypeStruct, Name: "Text", Module: "main", Identity: "main::Text"})
	voidType := types.Intern(Type{Kind: TypeVoid, Name: "void"})
	boxType := types.Intern(Type{Kind: TypeStruct, Name: "Box", Module: "main", Identity: "main::Box"})
	placeBox := types.Intern(Type{Kind: TypePlace, Element: boxType, Mutable: true})
	placeInt := types.Intern(Type{Kind: TypePlace, Element: intType, Mutable: true})
	refInt := types.Intern(Type{Kind: TypeReference, Element: intType})
	module := &Module{Version: Version, Identity: "main", Types: types,
		Structs: []StructDefinition{
			{TypeID: boxType, SymbolID: "main::Box", Name: "Box", CopyClassification: "move-only",
				Fields: []StructFieldDefinition{{ID: 0, Name: "Value", Type: textType}}},
			{TypeID: textType, SymbolID: "main::Text", Name: "Text", CopyClassification: "semantic"},
		},
		DestructionPlans: []DestructionPlanRecord{
			{ID: "destroy:main::Box", Type: boxType, Kind: "struct", Fields: []DestructionFieldRecord{{Field: 0, Plan: "destroy:main::Text"}}},
			{ID: "destroy:main::Text", Type: textType, Kind: "struct", CustomFree: true},
		},
	}
	v := func(id ValueID, typ TypeID, ownership OwnershipClass) Value {
		return Value{ID: id, Type: typ, Ownership: ownership}
	}
	owned, tmp, imm := OwnershipOwned, OwnershipCompilerTemporary, OwnershipImmediate
	boxes := &Function{ID: "main::Boxes(main::Box,main::Box)", Name: "Boxes", ReturnType: voidType,
		Parameters: []Parameter{{Name: "first", Value: v(1, boxType, owned)}, {Name: "second", Value: v(2, boxType, owned)}},
		Storages:   []Storage{{ID: 1, Name: "slot", Type: boxType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
	boxes.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpOwnMove, Operands: []ValueID{1}, Results: []Value{v(3, boxType, owned)}, Ownership: &OwnershipFacts{}},
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{3}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(4, placeBox, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpOwnMoveFromPlace, Operands: []ValueID{4}, Results: []Value{v(5, boxType, owned)}, Ownership: &OwnershipFacts{}},
		{Kind: OpOwnInitializePlace, Operands: []ValueID{4, 5}, Ownership: &OwnershipFacts{}},
		{Kind: OpOwnReplacePlace, Operands: []ValueID{4, 2}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Box", OldCleanup: 1, NewCleanup: 2}},
		{Kind: OpOwnDestroyPlace, Operands: []ValueID{4}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Box", Cause: "scope-exit"}},
		{Kind: OpReturn},
	}}}
	values := &Function{ID: "main::Values(main::Text,int,ref int)", Name: "Values", ReturnType: voidType,
		Parameters: []Parameter{{Name: "text", Value: v(1, textType, owned)}, {Name: "count", Value: v(2, intType, imm)},
			{Name: "view", Value: v(3, refInt, OwnershipSharedReference), Reference: p15Proven(1, 0)}},
		Storages: []Storage{{ID: 1, Name: "cell", Type: intType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
	values.Parameters[2].Reference.ProofProvenance, values.Parameters[2].Reference.Provenance = "caller-owned-borrow", "parameter"
	values.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpOwnSemanticCopy, Operands: []ValueID{1}, Results: []Value{v(4, textType, owned)}, Ownership: &OwnershipFacts{SemanticCopyPlan: "copy:main::Text"}},
		{Kind: OpOwnDiscard, Operands: []ValueID{4}, Ownership: &OwnershipFacts{DiscardOrigin: "explicit", DestructionPlan: "destroy:main::Text"}},
		{Kind: OpOwnDestroyValue, Operands: []ValueID{1}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Text", Cause: "function-exit"}},
		{Kind: OpOwnCopy, Operands: []ValueID{2}, Results: []Value{v(5, intType, imm)}, Ownership: &OwnershipFacts{}},
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{5}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(6, placeInt, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpOwnCopyFromPlace, Operands: []ValueID{6}, Results: []Value{v(7, intType, imm)}, Ownership: &OwnershipFacts{CopyMode: OwnCopyTrivial}},
		{Kind: OpOwnDiscard, Operands: []ValueID{7}, Ownership: &OwnershipFacts{DiscardOrigin: "compiler-temporary"}},
		{Kind: OpOwnDiscard, Operands: []ValueID{3}, Ownership: &OwnershipFacts{DiscardOrigin: "implicit-call-result"}},
		{Kind: OpReturn},
	}}}
	module.Functions = []*Function{boxes, values}
	return module
}

// P17-26 … P17-29: the ten ownership operations verify with their copy
// modes, destruction plans, causes, discard origins and cleanup identities,
// and print deterministically.
func TestOwnershipOperationsVerifyAndPrint(t *testing.T) {
	module := ownershipModule()
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if Format(module) != text {
		t.Fatal("ownership printing is not deterministic")
	}
	for _, want := range []string{
		`"destroy:main::Box" !4 struct`,
		"own.move %1", "own.initialize-place %4, %5", "own.move-from-place %4",
		`own.replace-place %4, %2 plan="destroy:main::Box" cleanup=#1->#2`,
		`own.destroy-place %4 plan="destroy:main::Box" cause=scope-exit`,
		`own.semantic-copy %1 copy-plan="copy:main::Text"`,
		`own.discard %4 plan="destroy:main::Text" origin=explicit`,
		`own.destroy-value %1 plan="destroy:main::Text" cause=function-exit`,
		"own.copy %2", "own.copy-from-place %6 mode=trivial", "own.discard %3 origin=implicit-call-result",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

// Initialization may acquire new cleanup responsibility without terminating
// an old incarnation. Rules: rules/mlir/packages/sec-mlir-dialect_package17.md
// — §54; cleanup registration production remains P17-33/35.
func TestOwnershipInitializationNewCleanup(t *testing.T) {
	module := ownershipModule()
	module.Functions[0].Blocks[0].Operations[5].Ownership.NewCleanup = 3
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// Each ownership contract rejects its violation with its own diagnostic.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§48–58.
func TestOwnershipOperationsRejectInvalidUse(t *testing.T) {
	op := func(m *Module, fn, index int) *Operation { return &m.Functions[fn].Blocks[0].Operations[index] }
	for _, test := range []struct {
		name, want string
		change     func(*Module)
	}{
		{"copy of move-only", "own.copy requires a copy-trivial type", func(m *Module) { op(m, 0, 0).Kind = OpOwnCopy }},
		{"semantic copy without plan", "resolved infallible semantic-copy plan", func(m *Module) { op(m, 1, 0).Ownership.SemanticCopyPlan = "" }},
		{"move changes type", "must keep its value type", func(m *Module) { op(m, 0, 0).Results[0].Type = 1 }},
		{"use after move", "after own.move consumed it", func(m *Module) { op(m, 0, 6).Operands[1] = 1 }},
		{"double destroy", "after own.destroy-value consumed it", func(m *Module) {
			fn := m.Functions[1]
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:3], append([]Operation{*op(m, 1, 2)}, fn.Blocks[0].Operations[3:]...)...)
		}},
		{"read-only place move", "requires a writable Place", func(m *Module) {
			m.Functions[0].Blocks[0].Operations[3].Results[0].Type = m.Types.Intern(Type{Kind: TypePlace, Element: 4})
		}},
		{"initialize with old destruction", "own.initialize-place must not destroy an old value", func(m *Module) { op(m, 0, 5).Ownership.DestructionPlan = "destroy:main::Box" }},
		{"initialize with old cleanup", "own.initialize-place must not destroy an old value", func(m *Module) { op(m, 0, 5).Ownership.OldCleanup = 1 }},
		{"replace without plan", "own.replace-place old value: missing destruction plan", func(m *Module) { op(m, 0, 6).Ownership.DestructionPlan = "" }},
		{"replace with same cleanup", "distinct old and new cleanup actions", func(m *Module) { op(m, 0, 6).Ownership.NewCleanup = 1 }},
		{"destroy with wrong plan", "destroys !2, not !4", func(m *Module) { op(m, 0, 7).Ownership.DestructionPlan = "destroy:main::Text" }},
		{"destroy without cause", "unknown cause", func(m *Module) { op(m, 0, 7).Ownership.Cause = "" }},
		{"unknown plan", "unknown destruction plan", func(m *Module) { op(m, 1, 2).Ownership.DestructionPlan = "destroy:missing" }},
		{"discard non-trivial without plan", "requires its destruction plan", func(m *Module) { op(m, 1, 1).Ownership.DestructionPlan = "" }},
		{"discard origin", "own.discard has unknown origin", func(m *Module) { op(m, 1, 1).Ownership.DiscardOrigin = "silent" }},
		{"discard reference destroys referent", "must not destroy its referent", func(m *Module) { op(m, 1, 9).Ownership.DestructionPlan = "destroy:main::Text" }},
		{"destroy reference", "borrow holders end with ref.end-borrow", func(m *Module) {
			*op(m, 1, 9) = Operation{Kind: OpOwnDestroyValue, Operands: []ValueID{3}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Text", Cause: "cleanup"}}
		}},
		{"copy-from-place mode", "unknown copy mode", func(m *Module) { op(m, 1, 7).Ownership.CopyMode = "" }},
		{"missing facts", "has no ownership facts", func(m *Module) { op(m, 0, 0).Ownership = nil }},
		{"duplicate plan", "duplicate destruction plan", func(m *Module) {
			m.DestructionPlans = append(m.DestructionPlans, m.DestructionPlans[0])
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := ownershipModule()
			test.change(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
