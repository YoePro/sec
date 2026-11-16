package semantic

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema"
)

// arenaOperationFixture represents a resolved P19 plan, not a source producer or
// provider implementation. Candidate values are used only on the success edge.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96.
func arenaOperationFixture(kind OpKind, width uint16) (*Module, *Operation) {
	m := &Module{Version: Version, Identity: "main", Types: NewTypeTable(), PointerWidthBits: width}
	types := m.Types
	arena := types.InternArena()
	void := types.Intern(Type{Kind: TypeVoid})
	boolean := types.Intern(Type{Kind: TypeBool})
	byteType := types.Intern(Type{Kind: TypeByte})
	uintType := types.Intern(Type{Kind: TypeUint, TargetSize: true})
	errorType := types.Intern(Type{Kind: TypeEnum, Name: "AllocationError", Identity: "core::AllocationError", Underlying: uintType})
	m.Enums = []EnumDefinition{{TypeID: errorType, SymbolID: "core::AllocationError", Name: "AllocationError", Underlying: uintType, RepresentationKind: EnumRepresentationInteger}}
	for index, name := range []string{"OutOfMemory", "Unsupported", "InvalidSize", "InvalidAlignment"} {
		m.Enums[0].Cases = append(m.Enums[0].Cases, EnumCase{ID: EnumCaseID(index), Name: name, Value: big.NewInt(int64(index))})
	}
	refArena := types.Intern(Type{Kind: TypeReference, Element: arena, Mutable: true})
	placeArena := types.Intern(Type{Kind: TypePlace, Element: arena, Mutable: true})
	sliceByte, _ := types.InternSlice(byteType, true)
	refByte := types.Intern(Type{Kind: TypeReference, Element: byteType, Mutable: true})
	ownerFacts := &ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: ReferenceProven, ProofProvenance: "caller-owned-borrow", AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "parameter"}
	sliceFacts := *ownerFacts
	sliceFacts.Borrow = 42
	sliceFacts.Lifetime = 42
	fn := &Function{ID: "main::F", Name: "F", ReturnType: void, Entry: 0, Parameters: []Parameter{
		{Name: "owner", Value: Value{ID: 0, Type: arena, Ownership: OwnershipOwned}},
		{Name: "control", Value: Value{ID: 1, Type: refArena, Ownership: OwnershipMutableReference}, Reference: ownerFacts},
		{Name: "count", Value: Value{ID: 2, Type: uintType, Ownership: OwnershipImmediate}},
		{Name: "backing", Value: Value{ID: 3, Type: sliceByte, Ownership: OwnershipMutableReference}, Reference: &sliceFacts},
	}}
	m.Functions = []*Function{fn}
	entry := &Block{ID: 0, Operations: []Operation{{Kind: OpReferenceDerefPlace, Operands: []ValueID{1}, Results: []Value{{ID: 4, Type: placeArena, Ownership: OwnershipCompilerTemporary}}, Reference: ownerFacts}}}
	fn.Blocks = []*Block{entry}
	f := &ArenaOperationFacts{StateBefore: 1, StateAfter: 2, EpochBefore: 7, EpochAfter: 7, BackingKind: "borrowed", GrowthKind: "fixed", CapacityPlan: "capacity", LayoutPlan: "layout", DefaultPlan: "default", DependencyProof: "dependency", RetainedBorrow: 42, FailureAtomic: true}
	b := &BackingMetadata{Domain: 5, Transition: string(sema.BackingTransitionNone), Origin: "Arena", Relation: string(sema.BackingRelationBorrowed), Authority: string(sema.ReclamationExternalOwner), Stability: "Stable", Space: "ordinary"}
	op := Operation{Kind: kind, Operands: []ValueID{4}, Arena: f, Backing: b}
	switch kind {
	case OpArenaCreateBorrowed, OpArenaCreateOwnedFixed, OpArenaCreateGrowable:
		f.StateBefore = 0
		f.EpochBefore = 0
		b.Transition = string(sema.BackingTransitionEstablishDomain)
		op.Operands = []ValueID{3}
		op.Results = []Value{{ID: 5, Type: arena, Ownership: OwnershipOwned}}
		if kind != OpArenaCreateBorrowed {
			op.Operands = []ValueID{2}
			f.BackingKind = "owned"
			f.RetainedBorrow = 0
			f.ProviderPlan = "provider"
			b.Relation = string(sema.BackingRelationOwned)
			b.Authority = string(sema.ReclamationOwningValue)
			if kind == OpArenaCreateGrowable {
				f.GrowthKind = "growable-nonrelocating"
			}
		}
	case OpArenaNew, OpArenaAlloc:
		candidateType := refByte
		if kind == OpArenaAlloc {
			candidateType = sliceByte
			op.Operands = append(op.Operands, 2)
		}
		op.Results = []Value{{ID: 5, Type: candidateType, Ownership: OwnershipMutableReference}}
		op.Reference = &ReferenceMetadata{Borrow: 6, Lifetime: 6, Validity: ReferenceDynamicEpoch, InvalidationDomain: 5, EpochDependency: 7, AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
	case OpArenaReset:
		f.EpochAfter = 8
		b.Transition = string(sema.BackingTransitionAdvanceEpoch)
	case OpArenaRelease, OpArenaDestroy:
		op.Operands = []ValueID{0}
		f.EpochAfter = 0
		b.Transition = string(sema.BackingTransitionEndDomain)
	}
	if kind == OpArenaCreateOwnedFixed || kind == OpArenaCreateGrowable {
		fn.RequiresAllocationContext = true
		context := types.InternAllocationContext()
		fn.Parameters = append([]Parameter{{Name: "$context", Value: Value{ID: 1000, Type: context, Ownership: OwnershipCompilerTemporary}, HiddenAllocationContext: true}}, fn.Parameters...)
		op.AllocationContext = &AllocationContextFacts{Value: 1000, Origin: ContextPropagated}
	}
	if kind == OpArenaDestroy {
		m.DestructionPlans = []DestructionPlanRecord{{ID: "destroy-Arena", Type: arena, Kind: "compiler-known-resource", Resource: "Arena"}}
		op.Ownership = &OwnershipFacts{DestructionPlan: "destroy-Arena", Cause: "scope-exit"}
	}
	if arenaCheckedOperation(kind) {
		op.Results = append(op.Results, Value{ID: 6, Type: boolean, Ownership: OwnershipImmediate}, Value{ID: 7, Type: errorType, Ownership: OwnershipImmediate})
	}
	entry.Operations = append(entry.Operations, op)
	if arenaCheckedOperation(kind) {
		entry.Operations = append(entry.Operations, Operation{Kind: OpCondBranch, Operands: []ValueID{6}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}})
		success := &Block{ID: 2}
		if kind == OpArenaNew || kind == OpArenaAlloc {
			success.Operations = append(success.Operations, Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{5}, Reference: op.Reference})
		} else {
			terminal := *f
			terminal.StateBefore = f.StateAfter
			terminal.StateAfter++
			terminal.EpochBefore = f.EpochAfter
			terminal.EpochAfter = 0
			backing := *b
			backing.Transition = string(sema.BackingTransitionEndDomain)
			success.Operations = append(success.Operations, Operation{Kind: OpArenaRelease, Operands: []ValueID{5}, Arena: &terminal, Backing: &backing})
		}
		success.Operations = append(success.Operations, Operation{Kind: OpReturn})
		fn.Blocks = append(fn.Blocks, &Block{ID: 1, Operations: []Operation{{Kind: OpReturn}}}, success)
	} else {
		if kind == OpArenaCreateBorrowed {
			terminal := *f
			terminal.StateBefore = f.StateAfter
			terminal.StateAfter++
			terminal.EpochBefore = f.EpochAfter
			terminal.EpochAfter = 0
			backing := *b
			backing.Transition = string(sema.BackingTransitionEndDomain)
			entry.Operations = append(entry.Operations, Operation{Kind: OpArenaRelease, Operands: []ValueID{5}, Arena: &terminal, Backing: &backing})
		}
		entry.Operations = append(entry.Operations, Operation{Kind: OpReturn})
	}
	return m, &entry.Operations[1]
}

// All eight operations verify through the public module boundary; all scalar
// plans keep the same semantic state/epoch and deterministic printer contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96.
func TestArenaOperations(t *testing.T) {
	for _, kind := range []OpKind{OpArenaCreateBorrowed, OpArenaCreateOwnedFixed, OpArenaCreateGrowable, OpArenaNew, OpArenaAlloc, OpArenaReset, OpArenaRelease, OpArenaDestroy} {
		for _, width := range []uint16{0, 32, 64} {
			t.Run(fmt.Sprintf("%s/%d", kind, width), func(t *testing.T) {
				m, op := arenaOperationFixture(kind, width)
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				text := Format(m)
				if !strings.Contains(text, string(kind)) || !strings.Contains(text, "state=#") || !strings.Contains(text, "epoch=#") || text != Format(m) {
					t.Fatal(text)
				}
				if !isArenaOperation(op.Kind) {
					t.Fatal("operation is not registered")
				}
			})
		}
	}
}

// Malformed IR cannot change receiver authority, source count units, element
// safety, ownership, canonical domain transitions or failure publication.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96.
func TestArenaOperationContracts(t *testing.T) {
	tests := []struct {
		name   string
		kind   OpKind
		mutate func(*Module, *Operation)
	}{
		{"destroy lacks P17 plan", OpArenaDestroy, func(m *Module, o *Operation) { o.Ownership = nil }},
		{"destroy names wrong resource", OpArenaDestroy, func(m *Module, o *Operation) { m.DestructionPlans[0].Resource = "Other" }},
		{"Arena facts on unrelated op", OpArenaReset, func(m *Module, o *Operation) { o.Kind = OpOwnMove }},
		{"fixed-width count", OpArenaAlloc, func(m *Module, o *Operation) {
			m.Functions[0].Parameters[2].Value.Type = m.Types.Intern(Type{Kind: TypeUint, BitWidth: 64})
		}},
		{"second release across blocks", OpArenaRelease, func(m *Module, o *Operation) {
			copy := *o
			block := m.Functions[0].Blocks[0]
			block.Operations[2] = Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 1}}}
			m.Functions[0].Blocks = append(m.Functions[0].Blocks, &Block{ID: 1, Operations: []Operation{copy, {Kind: OpReturn}}})
		}},
		{"borrowed backing reused", OpArenaCreateBorrowed, func(m *Module, o *Operation) {
			copy := *o
			block := m.Functions[0].Blocks[0]
			block.Operations = append(block.Operations[:3], copy, Operation{Kind: OpReturn})
		}},

		{"wrong retained backing borrow", OpArenaCreateBorrowed, func(m *Module, o *Operation) { o.Arena.RetainedBorrow++ }},
		{"failure edge forwards candidate", OpArenaCreateOwnedFixed, func(m *Module, o *Operation) {
			fn := m.Functions[0]
			v := o.Results[0]
			v.ID = 8
			fn.Blocks[1].Parameters = []Value{v}
			fn.Blocks[0].Operations[2].Successors[0].Arguments = []ValueID{o.Results[0].ID}
		}},

		{"missing facts", OpArenaNew, func(m *Module, o *Operation) { o.Arena = nil }},
		{"missing domain", OpArenaNew, func(m *Module, o *Operation) { o.Backing.Domain = 0 }},
		{"read-only place", OpArenaNew, func(m *Module, o *Operation) {
			id := m.Functions[0].Blocks[0].Operations[0].Results[0].Type
			m.Types.types[id-1].Mutable = false
		}},
		{"owner value instead of place", OpArenaAlloc, func(m *Module, o *Operation) { o.Operands[0] = 0 }},
		{"signed count", OpArenaAlloc, func(m *Module, o *Operation) {
			p := &m.Functions[0].Parameters[2]
			p.Value.Type = m.Types.Intern(Type{Kind: TypeInt, TargetSize: true})
		}},
		{"allocation advances epoch", OpArenaNew, func(m *Module, o *Operation) { o.Arena.EpochAfter++ }},
		{"stale result epoch", OpArenaAlloc, func(m *Module, o *Operation) { o.Reference.EpochDependency++ }},
		{"wrong result domain", OpArenaNew, func(m *Module, o *Operation) { o.Reference.InvalidationDomain++ }},
		{"unchanged reset epoch", OpArenaReset, func(m *Module, o *Operation) { o.Arena.EpochAfter = o.Arena.EpochBefore }},
		{"missing dependency proof", OpArenaReset, func(m *Module, o *Operation) { o.Arena.DependencyProof = "" }},
		{"release retains live epoch", OpArenaRelease, func(m *Module, o *Operation) { o.Arena.EpochAfter = 9 }},
		{"release place instead of owner", OpArenaRelease, func(m *Module, o *Operation) { o.Operands[0] = 4 }},
		{"release unowned value", OpArenaDestroy, func(m *Module, o *Operation) { m.Functions[0].Parameters[0].Value.Ownership = OwnershipImmediate }},
		{"creation copies arena", OpArenaCreateBorrowed, func(m *Module, o *Operation) { o.Results[0].Ownership = OwnershipImmediate }},
		{"backing borrow not retained", OpArenaCreateBorrowed, func(m *Module, o *Operation) { o.Arena.RetainedBorrow = 0 }},
		{"shared backing slice", OpArenaCreateBorrowed, func(m *Module, o *Operation) {
			p := &m.Functions[0].Parameters[3]
			p.Value.Type = m.Types.Intern(Type{Kind: TypeSharedSlice, Element: m.Types.Intern(Type{Kind: TypeByte})})
		}},
		{"borrowed backing grows", OpArenaCreateBorrowed, func(m *Module, o *Operation) { o.Arena.GrowthKind = "growable-nonrelocating" }},
		{"owned creation lacks provider", OpArenaCreateOwnedFixed, func(m *Module, o *Operation) { o.Arena.ProviderPlan = "" }},
		{"growable creation relocates", OpArenaCreateGrowable, func(m *Module, o *Operation) { o.Arena.GrowthKind = "fixed" }},
		{"non-atomic allocation", OpArenaNew, func(m *Module, o *Operation) { o.Arena.FailureAtomic = false }},
		{"missing initialization", OpArenaAlloc, func(m *Module, o *Operation) { o.Arena.DefaultPlan = "" }},
		{"missing layout", OpArenaNew, func(m *Module, o *Operation) { o.Arena.LayoutPlan = "" }},
		{"missing capacity", OpArenaAlloc, func(m *Module, o *Operation) { o.Arena.CapacityPlan = "" }},
		{"wrong transition", OpArenaReset, func(m *Module, o *Operation) { o.Backing.Transition = string(sema.BackingTransitionNone) }},
		{"state conflated with epoch", OpArenaAlloc, func(m *Module, o *Operation) { o.Arena.StateAfter = o.Arena.StateBefore }},
		{"nontrivial element", OpArenaNew, func(m *Module, o *Operation) {
			o.Results[0].Type = m.Types.Intern(Type{Kind: TypeReference, Element: m.Types.InternArena(), Mutable: true})
		}},
		{"candidate used on failure", OpArenaAlloc, func(m *Module, o *Operation) {
			b := m.Functions[0].Blocks[1]
			b.Operations = append([]Operation{{Kind: OpReferenceEndBorrow, Operands: []ValueID{5}, Reference: o.Reference}}, b.Operations...)
		}},
		{"candidate used after join", OpArenaNew, func(m *Module, o *Operation) {
			fn := m.Functions[0]
			fn.Blocks[1].Operations = []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}
			fn.Blocks[2].Operations = []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}
			fn.Blocks = append(fn.Blocks, &Block{ID: 3, Operations: []Operation{{Kind: OpReferenceEndBorrow, Operands: []ValueID{5}, Reference: o.Reference}, {Kind: OpReturn}}})
		}},
		{"branch tests candidate instead of failed", OpArenaNew, func(m *Module, o *Operation) { m.Functions[0].Blocks[0].Operations[2].Operands[0] = 5 }},
		{"second release", OpArenaRelease, func(m *Module, o *Operation) {
			block := m.Functions[0].Blocks[0]
			block.Operations = append(block.Operations[:2], *o, Operation{Kind: OpReturn})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, op := arenaOperationFixture(test.kind, 64)
			test.mutate(m, op)
			if err := Verify(m); err == nil {
				t.Fatal("invalid Arena operation accepted")
			}
		})
	}
}

// The existing P15 dynamic validity CFG guards an actual ArenaNewOp candidate.
// This is operation integration, not the still-open P19-18 source milestone.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§92,102;
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§59–63.
func TestArenaCandidateUsesReferenceValidityCFG(t *testing.T) {
	m, op := arenaOperationFixture(OpArenaNew, 64)
	fn := m.Functions[0]
	owner := &builder{module: m, maxPackage: 19}
	fb := &functionBuilder{owner: owner, fn: fn, nextValue: 8, nextBlock: 3, current: fn.Blocks[2]}
	fb.current.Operations = nil
	reference := builtValue{id: op.Results[0].ID, typ: op.Results[0].Type}
	guard, err := fb.validateReference(reference, op.Reference, Location{})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := m.Types.Lookup(reference.typ)
	place := m.Types.Intern(Type{Kind: TypePlace, Element: result.Element, Mutable: true})
	fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: op.Reference, ReferenceGuard: guard}, place)
	fb.emit(Operation{Kind: OpReturn})
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, block := range fn.Blocks {
		for i := range block.Operations {
			if block.Operations[i].Kind == OpReferenceDerefPlace && block.Operations[i].Operands[0] == reference.id {
				block.Operations[i].ReferenceGuard = 0
			}
		}
	}
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "requires its ref.is-valid guard") {
		t.Fatalf("unguarded Arena candidate verified: %v", err)
	}
}

// Each creation is the sole definition of its canonical domain identity.
// Rules: rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§3,6–7,26.
func TestArenaCreationDomainFreshness(t *testing.T) {
	m, _ := arenaOperationFixture(OpArenaCreateBorrowed, 64)
	other, _ := arenaOperationFixture(OpArenaCreateBorrowed, 64)
	other.Functions[0].ID = "main::G"
	other.Functions[0].Name = "G"
	m.Functions = append(m.Functions, other.Functions[0])
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "reuses domain") {
		t.Fatalf("reused domain accepted: %v", err)
	}
}

// Passing an owned candidate on the success edge is valid; its failure edge
// carries no candidate. Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§90–91.
func TestArenaCandidateSuccessArgument(t *testing.T) {
	m, op := arenaOperationFixture(OpArenaCreateOwnedFixed, 64)
	fn := m.Functions[0]
	candidate := op.Results[0]
	candidate.ID = 8
	fn.Blocks[2].Parameters = []Value{candidate}
	fn.Blocks[2].Operations[0].Operands[0] = candidate.ID
	fn.Blocks[0].Operations[2].Successors[1].Arguments = []ValueID{op.Results[0].ID}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}
