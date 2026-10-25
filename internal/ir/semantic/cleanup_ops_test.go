package semantic

import (
	"strings"
	"testing"
)

// cleanupModule follows package17 § 41: `first` is tracked after its
// initialization, a defer capturing first's Place registers next, then
// `second` is tracked; second's ownership moves out (its cleanup is
// canceled and the moved value destroyed), an inner scope exits, and the
// normal return runs the remaining registrations before returning.
func cleanupModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", BitWidth: 64})
	stringType := types.Intern(Type{Kind: TypeString, Name: "string"})
	voidType := types.Intern(Type{Kind: TypeVoid, Name: "void"})
	boxType := types.Intern(Type{Kind: TypeStruct, Name: "Box", Module: "main", Identity: "main::Box"})
	placeBox := types.Intern(Type{Kind: TypePlace, Element: boxType, Mutable: true})
	placeInt := types.Intern(Type{Kind: TypePlace, Element: intType, Mutable: true})
	module := &Module{Version: Version, Identity: "main", Types: types,
		Structs: []StructDefinition{{TypeID: boxType, SymbolID: "main::Box", Name: "Box", CopyClassification: "move-only",
			Fields: []StructFieldDefinition{{ID: 0, Name: "Name", Type: stringType}, {ID: 1, Name: "Count", Type: intType}}}},
		DestructionPlans: []DestructionPlanRecord{
			{ID: "destroy:main::Box", Type: boxType, Kind: "struct", Fields: []DestructionFieldRecord{{Field: 0, Plan: "destroy:string"}}},
			{ID: "destroy:string", Type: stringType, Kind: "compiler-known-resource", Resource: "string-release"},
			{ID: "destroy:int", Type: intType, Kind: "trivial"},
		},
	}
	v := func(id ValueID, typ TypeID, ownership OwnershipClass) Value {
		return Value{ID: id, Type: typ, Ownership: ownership}
	}
	owned, tmp, imm := OwnershipOwned, OwnershipCompilerTemporary, OwnershipImmediate
	fn := &Function{ID: "main::Order(main::Box,main::Box)", Name: "Order", ReturnType: voidType,
		Parameters: []Parameter{{Name: "first", Value: v(1, boxType, owned)}, {Name: "second", Value: v(2, boxType, owned)}},
		Storages: []Storage{
			{ID: 1, Name: "first", Type: boxType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true},
			{ID: 2, Name: "second", Type: boxType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true},
		}}
	body := &Region{Entry: 0, Blocks: []*Block{{ID: 0, Parameters: []Value{v(20, placeBox, tmp)}, Operations: []Operation{
		{Kind: OpPlaceField, Operands: []ValueID{20}, StructField: 1, Results: []Value{v(21, placeInt, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpPlaceRead, Operands: []ValueID{21}, Results: []Value{v(22, intType, imm)}},
		{Kind: OpOwnDiscard, Operands: []ValueID{22}, Ownership: &OwnershipFacts{DiscardOrigin: "compiler-temporary"}},
		{Kind: OpCleanupDeferYield, Cleanup: &CleanupFacts{}},
	}}}}
	fn.Blocks = []*Block{{ID: 0, Operations: []Operation{
		{Kind: OpStorageDeclare, Storage: 1},
		{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{1}},
		{Kind: OpPlaceStorageRoot, Storage: 1, Results: []Value{v(3, placeBox, tmp)}, Place: p15Place(1, PlaceRootLocal)},
		{Kind: OpCleanupTrackOwned, Operands: []ValueID{3}, Cleanup: &CleanupFacts{Action: 1, Scope: 1, Ordinal: 1, DestructionPlan: "destroy:main::Box"}},
		{Kind: OpCleanupDeferRegister, Operands: []ValueID{3}, Cleanup: &CleanupFacts{Action: 2, Scope: 1, Ordinal: 2, Defer: 1}, Region: body},
		{Kind: OpStorageDeclare, Storage: 2},
		{Kind: OpStorageInit, Storage: 2, Operands: []ValueID{2}},
		{Kind: OpPlaceStorageRoot, Storage: 2, Results: []Value{v(4, placeBox, tmp)}, Place: p15Place(2, PlaceRootLocal)},
		{Kind: OpCleanupTrackOwned, Operands: []ValueID{4}, Cleanup: &CleanupFacts{Action: 3, Scope: 2, Ordinal: 3, DestructionPlan: "destroy:main::Box"}},
		{Kind: OpOwnMoveFromPlace, Operands: []ValueID{4}, Results: []Value{v(5, boxType, owned)}, Ownership: &OwnershipFacts{}},
		{Kind: OpCleanupCancel, Cleanup: &CleanupFacts{Action: 3, Reason: "moved"}},
		{Kind: OpOwnDestroyValue, Operands: []ValueID{5}, Ownership: &OwnershipFacts{DestructionPlan: "destroy:main::Box", Cause: "compiler-generated"}},
		{Kind: OpCleanupRunScope, Cleanup: &CleanupFacts{Scopes: []uint32{2}, Exit: "fallthrough"}},
		{Kind: OpCleanupRunFunction, Cleanup: &CleanupFacts{Exit: "return"}},
		{Kind: OpReturn},
	}}}
	module.Functions = []*Function{fn}
	return module
}

// P17-33: the six cleanup operations verify and print, including the
// isolated defer region over its capture Place.
func TestCleanupOperationsVerifyAndPrint(t *testing.T) {
	module := cleanupModule()
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if Format(module) != text {
		t.Fatal("cleanup printing is not deterministic")
	}
	for _, want := range []string{
		`cleanup.track-owned %3 action=#1 scope=#1 ordinal=1 plan="destroy:main::Box"`,
		"cleanup.defer-register %3 action=#2 scope=#1 ordinal=2 defer=#1 {",
		"^0(%20: !",
		"cleanup.defer-yield",
		"cleanup.cancel action=#3 reason=moved",
		"cleanup.run-scope scopes=[#2] exit=fallthrough",
		"cleanup.run-function exit=return",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestCleanupOperationsRejectInvalidUse(t *testing.T) {
	op := func(m *Module, index int) *Operation { return &m.Functions[0].Blocks[0].Operations[index] }
	region := func(m *Module) *Region { return op(m, 4).Region }
	for _, test := range []struct {
		name, want string
		change     func(*Module)
	}{
		{"duplicate action", "reuses cleanup action #1", func(m *Module) { op(m, 8).Cleanup.Action = 1 }},
		{"duplicate ordinal", "or ordinal 2", func(m *Module) { op(m, 8).Cleanup.Ordinal = 2 }},
		{"cancel unregistered", "cleanup.cancel of unregistered action #9", func(m *Module) { op(m, 10).Cleanup.Action = 9 }},
		{"cancel reason", "requires an action identity and a reason", func(m *Module) { op(m, 10).Cleanup.Reason = "forgot" }},
		{"track trivial", "registers nothing", func(m *Module) {
			fn := m.Functions[0]
			placeInt := region(m).Blocks[0].Operations[0].Results[0].Type
			field := Operation{Kind: OpPlaceField, Operands: []ValueID{3}, StructField: 1, Results: []Value{{ID: 30, Type: placeInt, Ownership: OwnershipCompilerTemporary}}, Place: p15Place(1, PlaceRootLocal)}
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:3], append([]Operation{field}, fn.Blocks[0].Operations[3:]...)...)
			op(m, 4).Operands, op(m, 4).Cleanup.DestructionPlan = []ValueID{30}, "destroy:int"
		}},
		{"track without plan", "cleanup.track-owned: missing destruction plan", func(m *Module) { op(m, 3).Cleanup.DestructionPlan = "" }},
		{"defer captures value", "which is not a Place", func(m *Module) { op(m, 4).Operands = []ValueID{1} }},
		{"defer without region", "has no body region", func(m *Module) { op(m, 4).Region = nil }},
		{"region parameter mismatch", "does not match capture %3", func(m *Module) { region(m).Blocks[0].Parameters[0].Type = 1 }},
		{"region uses outer value", "undefined operand %5", func(m *Module) { region(m).Blocks[0].Operations[2].Operands = []ValueID{5} }},
		{"region returns", "must end with cleanup.defer-yield", func(m *Module) {
			region(m).Blocks[0].Operations[3] = Operation{Kind: OpReturn}
		}},
		{"yield outside region", "cleanup.defer-yield outside a defer region", func(m *Module) {
			fn := m.Functions[0]
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:13], Operation{Kind: OpCleanupDeferYield, Cleanup: &CleanupFacts{}})
		}},
		{"run-function not before return", "must precede the return of its block", func(m *Module) {
			fn := m.Functions[0]
			ops := fn.Blocks[0].Operations
			ops[12], ops[13] = ops[13], ops[12]
			fn.Blocks[0].Operations = append(ops[:13], Operation{Kind: OpCleanupRunScope, Cleanup: &CleanupFacts{Scopes: []uint32{1}, Exit: "fallthrough"}}, ops[14])
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:12], fn.Blocks[0].Operations[13:]...)
			fn.Blocks[0].Operations = append(fn.Blocks[0].Operations[:len(fn.Blocks[0].Operations)-1],
				Operation{Kind: OpCleanupRunFunction, Cleanup: &CleanupFacts{Exit: "return"}}, Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 1}}})
			fn.Blocks = append(fn.Blocks, &Block{ID: 1, Operations: []Operation{{Kind: OpReturn}}})
		}},
		{"run-scope without scopes", "requires exited scopes", func(m *Module) { op(m, 12).Cleanup.Scopes = nil }},
		{"run-scope exit kind", "requires exited scopes and a scope exit kind", func(m *Module) { op(m, 12).Cleanup.Exit = "return" }},
		{"run-function with scopes", "function exit kind and no scopes", func(m *Module) { op(m, 13).Cleanup.Scopes = []uint32{1} }},
		{"missing facts", "has no cleanup facts", func(m *Module) { op(m, 13).Cleanup = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := cleanupModule()
			test.change(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
