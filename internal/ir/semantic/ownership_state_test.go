package semantic

import (
	"strings"
	"testing"
)

// ownershipStateModule has a complete destruction-plan graph — a struct
// destroying its two string fields in reverse declaration order, a
// 1000-element string array destroyed by one element plan and count, an
// active-variant union, a custom-free struct and the string resource — and
// two functions: Partial moves `left` out of a pair and destroys the pair
// skipping it; Reinit moves `left` out, reinitializes it and destroys the
// complete pair.
func ownershipStateModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", BitWidth: 64})
	stringType := types.Intern(Type{Kind: TypeString, Name: "string"})
	textType := types.Intern(Type{Kind: TypeStruct, Name: "Text", Module: "main", Identity: "main::Text"})
	voidType := types.Intern(Type{Kind: TypeVoid, Name: "void"})
	pairType := types.Intern(Type{Kind: TypeStruct, Name: "Pair", Module: "main", Identity: "main::Pair"})
	handleType := types.Intern(Type{Kind: TypeStruct, Name: "Handle", Module: "main", Identity: "main::Handle"})
	arrayType := types.Intern(Type{Kind: TypeArray, Element: textType, Length: "1000"})
	maybeType := types.Intern(Type{Kind: TypeUnion, Name: "Maybe", Module: "main", Identity: "main::Maybe"})
	placePair := types.Intern(Type{Kind: TypePlace, Element: pairType, Mutable: true})
	placeHandle := types.Intern(Type{Kind: TypePlace, Element: handleType, Mutable: true})
	placeString := types.Intern(Type{Kind: TypePlace, Element: textType, Mutable: true})
	module := &Module{Version: Version, Identity: "main", Types: types,
		Structs: []StructDefinition{
			{TypeID: pairType, SymbolID: "main::Pair", Name: "Pair", CopyClassification: "move-only",
				Fields: []StructFieldDefinition{{ID: 0, Name: "left", Type: textType}, {ID: 1, Name: "right", Type: textType}, {ID: 2, Name: "count", Type: intType}}},
			{TypeID: handleType, SymbolID: "main::Handle", Name: "Handle", CopyClassification: "move-only",
				Fields: []StructFieldDefinition{{ID: 0, Name: "name", Type: textType}}},
			{TypeID: textType, SymbolID: "main::Text", Name: "Text", CopyClassification: "move-only"},
		},
		Unions: []UnionDefinition{{TypeID: maybeType, SymbolID: "main::Maybe", Name: "Maybe", CopyClassification: "move-only",
			Variants: []UnionVariantDefinition{{Index: 0, Name: "Some", Kind: UnionVariantSingle, Payload: textType}, {Index: 1, Name: "None", Kind: UnionVariantEmpty}}}},
		DestructionPlans: []DestructionPlanRecord{
			{ID: "destroy:main::Pair", Type: pairType, Kind: "struct", Fields: []DestructionFieldRecord{{Field: 1, Plan: "destroy:main::Text"}, {Field: 0, Plan: "destroy:main::Text"}}},
			{ID: "destroy:main::Handle", Type: handleType, Kind: "struct", CustomFree: true, Fields: []DestructionFieldRecord{{Field: 0, Plan: "destroy:main::Text"}}},
			{ID: "destroy:string[1000]", Type: arrayType, Kind: "fixed-array", Element: "destroy:main::Text", ElementCount: "1000"},
			{ID: "destroy:main::Maybe", Type: maybeType, Kind: "active-variant", Variants: []DestructionVariantRecord{{Variant: 0, Plan: "destroy:main::Text"}}},
			{ID: "destroy:main::Text", Type: textType, Kind: "struct", CustomFree: true},
			{ID: "destroy:string", Type: stringType, Kind: "compiler-known-resource", Resource: "string-release"},
			{ID: "destroy:int", Type: intType, Kind: "trivial"},
		},
	}
	v := func(id ValueID, typ TypeID, ownership OwnershipClass) Value {
		return Value{ID: id, Type: typ, Ownership: ownership}
	}
	owned, tmp := OwnershipOwned, OwnershipCompilerTemporary
	pairFunction := func(name string, reinit bool) *Function {
		fn := &Function{ID: FunctionID("main::" + name), Name: name, ReturnType: voidType,
			Parameters: []Parameter{{Name: "pair", Value: v(1, pairType, owned)}, {Name: "text", Value: v(2, textType, owned)}},
			Storages:   []Storage{{ID: 1, Name: "slot", Type: pairType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
		ops := []Operation{
			{Kind: OpStorageDeclare, Storage: 1},
			{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{1}},
			{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(3, placePair, tmp)}, Place: p15Place(1, PlaceRootLocal)},
			{Kind: OpPlaceField, Operands: []ValueID{3}, StructField: 0, Results: []Value{v(4, placeString, tmp)}, Place: p15Place(1, PlaceRootLocal)},
			{Kind: OpOwnMoveFromPlace, Operands: []ValueID{4}, Results: []Value{v(5, textType, owned)}, Ownership: &OwnershipFacts{}},
			{Kind: OpOwnDiscard, Operands: []ValueID{5}, Ownership: &OwnershipFacts{DiscardOrigin: "explicit", DestructionPlan: "destroy:main::Text"}},
		}
		destroy := &OwnershipFacts{DestructionPlan: "destroy:main::Pair", Cause: "scope-exit", Skipped: []OwnedPath{{{Index: 0}}}}
		if reinit {
			ops = append(ops, Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{4, 2}, Ownership: &OwnershipFacts{NewCleanup: 3}})
			destroy.Skipped = nil
		} else {
			ops = append(ops, Operation{Kind: OpOwnDestroyValue, Operands: []ValueID{2}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Text", Cause: "function-exit"}})
		}
		ops = append(ops, Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{3}, Ownership: destroy}, Operation{Kind: OpReturn})
		fn.Blocks = []*Block{{ID: 0, Operations: ops}}
		return fn
	}
	handle := &Function{ID: "main::Handle(main::Handle)", Name: "HandleUse", ReturnType: voidType,
		Parameters: []Parameter{{Name: "handle", Value: v(1, handleType, owned)}},
		Storages:   []Storage{{ID: 1, Name: "slot", Type: handleType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true}}}
	handle.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{1}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(2, placeHandle, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpOwnDestroyPlace, Operands: []ValueID{2}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Handle", Cause: "scope-exit"}},
		{Kind: OpReturn},
	}}}
	module.Functions = []*Function{pairFunction("Partial", false), pairFunction("Reinit", true), handle}
	return module
}

// P17-30: the derived destruction graph verifies and prints compactly — the
// 1000-element array is one record — and partial moves are skipped by the
// destroy or reinitialized before a complete destroy.
func TestOwnershipDerivedDestructionAndPartialState(t *testing.T) {
	module := ownershipStateModule()
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`"destroy:main::Pair" !5 struct field#1="destroy:main::Text" field#0="destroy:main::Text"`,
		`"destroy:main::Handle" !6 struct custom-free field#0="destroy:main::Text"`,
		`"destroy:string[1000]" !7 fixed-array element="destroy:main::Text" count=1000`,
		`"destroy:main::Maybe" !8 active-variant variant#0="destroy:main::Text"`,
		`own.destroy-place %3 plan="destroy:main::Pair" cause=scope-exit skip=[field#0]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "destroy:string[1000]") != 1 {
		t.Fatalf("array destruction must stay one compact record:\n%s", text)
	}
}

func TestOwnershipDerivedDestructionAndPartialStateRejects(t *testing.T) {
	op := func(m *Module, fn, index int) *Operation { return &m.Functions[fn].Blocks[0].Operations[index] }
	plan := func(m *Module, id string) *DestructionPlanRecord {
		for index := range m.DestructionPlans {
			if m.DestructionPlans[index].ID == id {
				return &m.DestructionPlans[index]
			}
		}
		return nil
	}
	for _, test := range []struct {
		name, want string
		change     func(*Module)
	}{
		{"destroy without skipping moved field", "does not skip moved-out subobject \"field#0\"", func(m *Module) { op(m, 0, 7).Ownership.Skipped = nil }},
		{"second move of moved field", "overlaps moved-out subobject \"field#0\"", func(m *Module) {
			fn := m.Functions[0]
			second := *op(m, 0, 4)
			second.Results = []Value{{ID: 9, Type: second.Results[0].Type, Ownership: OwnershipOwned}}
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:5], append([]Operation{second}, fn.Blocks[0].Operations[5:]...)...)
		}},
		{"whole move after partial move", "overlaps moved-out subobject \"field#0\"", func(m *Module) {
			whole := Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{3}, Results: []Value{{ID: 9, Type: 5, Ownership: OwnershipOwned}}, Ownership: &OwnershipFacts{}}
			fn := m.Functions[0]
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:5], append([]Operation{whole}, fn.Blocks[0].Operations[5:]...)...)
		}},
		{"skip through custom free", "custom free cannot be partially moved", func(m *Module) { plan(m, "destroy:main::Pair").CustomFree = true }},
		{"plan omits non-trivial field", "omits non-trivial field #0", func(m *Module) { plan(m, "destroy:main::Pair").Fields = plan(m, "destroy:main::Pair").Fields[:1] }},
		{"plan field order", "reverse declaration order", func(m *Module) {
			fields := plan(m, "destroy:main::Pair").Fields
			fields[0], fields[1] = fields[1], fields[0]
		}},
		{"plan steps to wrong type", "which destroys !1, not !3", func(m *Module) { plan(m, "destroy:main::Pair").Fields[0].Plan = "destroy:int" }},
		{"plan steps to unknown plan", "steps to unknown plan", func(m *Module) { plan(m, "destroy:main::Maybe").Variants[0].Plan = "destroy:missing" }},
		{"array count", "exact count", func(m *Module) { plan(m, "destroy:string[1000]").ElementCount = "999" }},
		{"variant omitted", "omits non-trivial variant #0", func(m *Module) { plan(m, "destroy:main::Maybe").Variants = nil }},
		{"trivial non-trivial", "is trivial for a type that needs destruction", func(m *Module) {
			plan(m, "destroy:main::Text").Kind, plan(m, "destroy:main::Text").CustomFree = "trivial", false
		}},
		{"custom free on resource", "custom free belongs to a struct plan", func(m *Module) { plan(m, "destroy:string").CustomFree = true }},
		{"resource without contract", "names no release contract", func(m *Module) { plan(m, "destroy:string").Resource = "" }},
		{"foreign steps", "carries steps of another kind", func(m *Module) { plan(m, "destroy:string").Element = "destroy:int" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := ownershipStateModule()
			test.change(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
