package secmlir

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

func arenaChecked(kind semantic.OpKind) bool {
	return kind == semantic.OpArenaCreateOwnedFixed || kind == semantic.OpArenaCreateGrowable || kind == semantic.OpArenaNew || kind == semantic.OpArenaAlloc
}

// arenaOperationFixture mirrors the verified Semantic IR fixture of
// internal/ir/semantic (arena_operations_test.go): one function per Arena
// operation with its success/failure CFG and terminal Release.
func arenaOperationFixture(kind semantic.OpKind, width uint16) (*semantic.Module, *semantic.Operation) {
	m := &semantic.Module{Version: semantic.Version, Identity: "main", Types: semantic.NewTypeTable(), PointerWidthBits: width}
	types := m.Types
	arena := types.InternArena()
	void := types.Intern(semantic.Type{Kind: semantic.TypeVoid})
	boolean := types.Intern(semantic.Type{Kind: semantic.TypeBool})
	byteType := types.Intern(semantic.Type{Kind: semantic.TypeByte})
	uintType := types.Intern(semantic.Type{Kind: semantic.TypeUint, TargetSize: true})
	errorType := types.Intern(semantic.Type{Kind: semantic.TypeEnum, Name: "AllocationError", Identity: "core::AllocationError", Underlying: uintType})
	m.Enums = []semantic.EnumDefinition{{TypeID: errorType, SymbolID: "core::AllocationError", Name: "AllocationError", Underlying: uintType, RepresentationKind: semantic.EnumRepresentationInteger}}
	for index, name := range []string{"OutOfMemory", "Unsupported", "InvalidSize", "InvalidAlignment"} {
		m.Enums[0].Cases = append(m.Enums[0].Cases, semantic.EnumCase{ID: semantic.EnumCaseID(index), Name: name, Value: big.NewInt(int64(index))})
	}
	refArena := types.Intern(semantic.Type{Kind: semantic.TypeReference, Element: arena, Mutable: true})
	placeArena := types.Intern(semantic.Type{Kind: semantic.TypePlace, Element: arena, Mutable: true})
	sliceByte, _ := types.InternSlice(byteType, true)
	refByte := types.Intern(semantic.Type{Kind: semantic.TypeReference, Element: byteType, Mutable: true})
	ownerFacts := &semantic.ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: semantic.ReferenceProven, ProofProvenance: "caller-owned-borrow", AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "parameter"}
	sliceFacts := *ownerFacts
	sliceFacts.Borrow = 42
	sliceFacts.Lifetime = 42
	fn := &semantic.Function{ID: "main::F", Name: "F", ReturnType: void, Entry: 0, Parameters: []semantic.Parameter{
		{Name: "owner", Value: semantic.Value{ID: 0, Type: arena, Ownership: semantic.OwnershipOwned}},
		{Name: "control", Value: semantic.Value{ID: 1, Type: refArena, Ownership: semantic.OwnershipMutableReference}, Reference: ownerFacts},
		{Name: "count", Value: semantic.Value{ID: 2, Type: uintType, Ownership: semantic.OwnershipImmediate}},
		{Name: "backing", Value: semantic.Value{ID: 3, Type: sliceByte, Ownership: semantic.OwnershipMutableReference}, Reference: &sliceFacts},
	}}
	m.Functions = []*semantic.Function{fn}
	entry := &semantic.Block{ID: 0, Operations: []semantic.Operation{{Kind: semantic.OpReferenceDerefPlace, Operands: []semantic.ValueID{1}, Results: []semantic.Value{{ID: 4, Type: placeArena, Ownership: semantic.OwnershipCompilerTemporary}}, Reference: ownerFacts}}}
	fn.Blocks = []*semantic.Block{entry}
	f := &semantic.ArenaOperationFacts{StateBefore: 1, StateAfter: 2, EpochBefore: 7, EpochAfter: 7, BackingKind: "borrowed", GrowthKind: "fixed", CapacityPlan: "capacity", LayoutPlan: "layout", DefaultPlan: "default", DependencyProof: "dependency", RetainedBorrow: 42, FailureAtomic: true}
	b := &semantic.BackingMetadata{Domain: 5, Transition: string(sema.BackingTransitionNone), Origin: "Arena", Relation: string(sema.BackingRelationBorrowed), Authority: string(sema.ReclamationExternalOwner), Stability: "Stable", Space: "ordinary"}
	op := semantic.Operation{Kind: kind, Operands: []semantic.ValueID{4}, Arena: f, Backing: b}
	switch kind {
	case semantic.OpArenaCreateBorrowed, semantic.OpArenaCreateOwnedFixed, semantic.OpArenaCreateGrowable:
		f.StateBefore = 0
		f.EpochBefore = 0
		b.Transition = string(sema.BackingTransitionEstablishDomain)
		op.Operands = []semantic.ValueID{3}
		op.Results = []semantic.Value{{ID: 5, Type: arena, Ownership: semantic.OwnershipOwned}}
		if kind != semantic.OpArenaCreateBorrowed {
			op.Operands = []semantic.ValueID{2}
			f.BackingKind = "owned"
			f.RetainedBorrow = 0
			f.ProviderPlan = "provider"
			b.Relation = string(sema.BackingRelationOwned)
			b.Authority = string(sema.ReclamationOwningValue)
			if kind == semantic.OpArenaCreateGrowable {
				f.GrowthKind = "growable-nonrelocating"
			}
		}
	case semantic.OpArenaNew, semantic.OpArenaAlloc:
		candidateType := refByte
		if kind == semantic.OpArenaAlloc {
			candidateType = sliceByte
			op.Operands = append(op.Operands, 2)
		}
		op.Results = []semantic.Value{{ID: 5, Type: candidateType, Ownership: semantic.OwnershipMutableReference}}
		op.Reference = &semantic.ReferenceMetadata{Borrow: 6, Lifetime: 6, Validity: semantic.ReferenceDynamicEpoch, InvalidationDomain: 5, EpochDependency: 7, AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
	case semantic.OpArenaReset:
		f.EpochAfter = 8
		b.Transition = string(sema.BackingTransitionAdvanceEpoch)
	case semantic.OpArenaRelease, semantic.OpArenaDestroy:
		op.Operands = []semantic.ValueID{0}
		f.EpochAfter = 0
		b.Transition = string(sema.BackingTransitionEndDomain)
	}
	if kind == semantic.OpArenaCreateOwnedFixed || kind == semantic.OpArenaCreateGrowable {
		fn.RequiresAllocationContext = true
		context := types.InternAllocationContext()
		fn.Parameters = append([]semantic.Parameter{{Name: "$context", Value: semantic.Value{ID: 1000, Type: context, Ownership: semantic.OwnershipCompilerTemporary}, HiddenAllocationContext: true}}, fn.Parameters...)
		op.AllocationContext = &semantic.AllocationContextFacts{Value: 1000, Origin: semantic.ContextPropagated}
	}
	if kind == semantic.OpArenaDestroy {
		m.DestructionPlans = []semantic.DestructionPlanRecord{{ID: "destroy-Arena", Type: arena, Kind: "compiler-known-resource", Resource: "Arena"}}
		op.Ownership = &semantic.OwnershipFacts{DestructionPlan: "destroy-Arena", Cause: "scope-exit"}
	}
	if arenaChecked(kind) {
		op.Results = append(op.Results, semantic.Value{ID: 6, Type: boolean, Ownership: semantic.OwnershipImmediate}, semantic.Value{ID: 7, Type: errorType, Ownership: semantic.OwnershipImmediate})
	}
	entry.Operations = append(entry.Operations, op)
	if arenaChecked(kind) {
		entry.Operations = append(entry.Operations, semantic.Operation{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{6}, Successors: []semantic.BranchTarget{{Block: 1}, {Block: 2}}})
		success := &semantic.Block{ID: 2}
		if kind == semantic.OpArenaNew || kind == semantic.OpArenaAlloc {
			success.Operations = append(success.Operations, semantic.Operation{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{5}, Reference: op.Reference})
		} else {
			terminal := *f
			terminal.StateBefore = f.StateAfter
			terminal.StateAfter++
			terminal.EpochBefore = f.EpochAfter
			terminal.EpochAfter = 0
			backing := *b
			backing.Transition = string(sema.BackingTransitionEndDomain)
			success.Operations = append(success.Operations, semantic.Operation{Kind: semantic.OpArenaRelease, Operands: []semantic.ValueID{5}, Arena: &terminal, Backing: &backing})
		}
		success.Operations = append(success.Operations, semantic.Operation{Kind: semantic.OpReturn})
		fn.Blocks = append(fn.Blocks, &semantic.Block{ID: 1, Operations: []semantic.Operation{{Kind: semantic.OpReturn}}}, success)
	} else {
		if kind == semantic.OpArenaCreateBorrowed {
			terminal := *f
			terminal.StateBefore = f.StateAfter
			terminal.StateAfter++
			terminal.EpochBefore = f.EpochAfter
			terminal.EpochAfter = 0
			backing := *b
			backing.Transition = string(sema.BackingTransitionEndDomain)
			entry.Operations = append(entry.Operations, semantic.Operation{Kind: semantic.OpArenaRelease, Operands: []semantic.ValueID{5}, Arena: &terminal, Backing: &backing})
		}
		entry.Operations = append(entry.Operations, semantic.Operation{Kind: semantic.OpReturn})
	}
	return m, &entry.Operations[1]
}

// P19 §§ 105-123: every verified Semantic IR Arena operation emits to schema
// 15 on 32- and 64-bit plans and passes the dialect and the Arena,
// allocation-context, place and reference verifiers; schema 14 rejects the
// Arena types by name.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 105-131.
func TestSchema15ArenaOperations(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	kinds := []semantic.OpKind{semantic.OpArenaCreateBorrowed, semantic.OpArenaCreateOwnedFixed, semantic.OpArenaCreateGrowable,
		semantic.OpArenaNew, semantic.OpArenaAlloc, semantic.OpArenaReset, semantic.OpArenaRelease, semantic.OpArenaDestroy}
	want := map[semantic.OpKind]string{
		semantic.OpArenaCreateBorrowed:   `"sec.arena.create_borrowed"`,
		semantic.OpArenaCreateOwnedFixed: `"sec.arena.create_owned_fixed"`,
		semantic.OpArenaCreateGrowable:   `growth_kind = "growable-nonrelocating"`,
		semantic.OpArenaNew:              `validity_policy = "dynamic-epoch"`,
		semantic.OpArenaAlloc:            `"sec.arena.alloc"`,
		semantic.OpArenaReset:            `transition = "advance-epoch"`,
		semantic.OpArenaRelease:          `transition = "end-domain"`,
		semantic.OpArenaDestroy:          `destruction_plan = "destroy-Arena", cause = "scope-exit"`,
	}
	for _, width := range []uint16{32, 64} {
		for _, kind := range kinds {
			t.Run(fmt.Sprintf("%s/%d", kind, width), func(t *testing.T) {
				module, _ := arenaOperationFixture(kind, width)
				if _, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 14}); err == nil || !strings.Contains(err.Error(), "schema 15") {
					t.Fatalf("schema 14 accepted an Arena: %v", err)
				}
				output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 15})
				if err != nil {
					t.Fatal(err)
				}
				text := string(output)
				if !strings.Contains(text, "sec.dialect_version = 15 : i32") || !strings.Contains(text, want[kind]) {
					t.Fatalf("schema-15 module lacks %q:\n%s", want[kind], text)
				}
				if arenaChecked(kind) && kind != semantic.OpArenaNew && kind != semantic.OpArenaAlloc &&
					(!strings.Contains(text, "{sec.hidden_allocation_context = true}") || !strings.Contains(text, "!sec.alloc_context")) {
					t.Fatalf("owned creation lacks its hidden context:\n%s", text)
				}
				verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "-", "-o", os.DevNull)
				verify.Stdin = bytes.NewReader(output)
				if result, err := verify.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s\n%s", err, result, text)
				}
			})
		}
	}
	_ = big.NewInt
	_ = sema.BackingTransitionNone
}

// P19 §§ 107-109, 119, 128: an entry root produces the target-provided
// context and forwards it as the first operand of a direct call into a
// function with a marked hidden context input; the allocation-context
// verifier accepts it.
func TestSchema15HiddenContextForwarding(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	types := semantic.NewTypeTable()
	context := types.InternAllocationContext()
	void := types.Intern(semantic.Type{Kind: semantic.TypeVoid})
	callee := &semantic.Function{ID: "main::Use", Name: "Use", RequiresAllocationContext: true, ReturnType: void,
		Parameters: []semantic.Parameter{{Name: "$context", HiddenAllocationContext: true, Value: semantic.Value{ID: 0, Type: context, Ownership: semantic.OwnershipCompilerTemporary}}},
		Blocks:     []*semantic.Block{{ID: 0, Operations: []semantic.Operation{{Kind: semantic.OpReturn}}}}}
	facts := &semantic.AllocationContextFacts{Value: 1, Origin: semantic.ContextTarget, OriginPlan: "target-entry"}
	root := &semantic.Function{ID: "main::Root", Name: "Root", ReturnType: void, Blocks: []*semantic.Block{{ID: 0, Operations: []semantic.Operation{
		{Kind: semantic.OpAllocationContextTarget, Results: []semantic.Value{{ID: 1, Type: context, Ownership: semantic.OwnershipCompilerTemporary}}, AllocationContext: facts},
		{Kind: semantic.OpDirectCall, Callee: "main::Use", Operands: []semantic.ValueID{1}, ArgumentActions: []semantic.ArgumentAction{semantic.ArgumentAllocationContext}, AllocationContext: facts},
		{Kind: semantic.OpReturn},
	}}}}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{callee, root}}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{`!sec.alloc_context {sec.hidden_allocation_context = true}`, `"sec.alloc_context.target"() <{origin_plan = "target-entry"}>`, `sec.argument_actions = ["allocation-context"]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "--sec-verify-allocation-contexts", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}
