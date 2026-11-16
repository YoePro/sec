package semantic

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema"
)

// nestedArenaFixture transfers an Arena-derived byte-slice parameter into a
// child with its own domain/epoch, then terminates that child. No source
// producer, physical allocation or declaration syntax is introduced.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§50,97.
func nestedArenaFixture(width uint16) (*Module, *Operation) {
	m, create := arenaOperationFixture(OpArenaCreateBorrowed, width)
	reference := m.Functions[0].Parameters[3].Reference
	reference.Validity = ReferenceDynamicEpoch
	reference.ProofProvenance = ""
	reference.InvalidationDomain = 9
	reference.EpochDependency = 11
	reference.Provenance = "arena"
	reference.OriginStorages = []uint32{7}
	create.Arena.Parent = &ArenaParentDependency{Storage: 7, Reference: *cloneReferenceMetadata(reference), StaticRange: &StaticSliceRange{Start: big.NewInt(0), EndExclusive: big.NewInt(4096)}}
	return m, create
}

// nestedParentReset represents the already resolved parent transition; it
// borrows the parent control Place and retains its backing and domain.
// Rules: rules/memory/arena.md — §§35–42,51(5).
func nestedParentReset(m *Module) Operation {
	template := m.Functions[0].Blocks[0].Operations[2]
	facts := *template.Arena
	facts.StateBefore = 3
	facts.StateAfter = 4
	facts.EpochBefore = 11
	facts.EpochAfter = 12
	backing := *template.Backing
	backing.Domain = 9
	backing.Transition = string(sema.BackingTransitionAdvanceEpoch)
	return Operation{Kind: OpArenaReset, Operands: []ValueID{4}, Arena: &facts, Backing: &backing}
}

// Nested creation preserves canonical parent identity, range and epoch across
// scalar plans. Releasing the child discharges the dependency and permits a
// later parent Reset without reclaiming the parent's individual allocation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §97;
// rules/memory/arena.md — §51.
func TestNestedArenaIR(t *testing.T) {
	for _, width := range []uint16{0, 32, 64} {
		m, create := nestedArenaFixture(width)
		block := m.Functions[0].Blocks[0]
		block.Operations = append(block.Operations[:3], nestedParentReset(m), Operation{Kind: OpReturn})
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		for _, want := range []string{"parent-storage=#7", "domain=#9 epoch=#11", "parent-range=[0,4096)"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		create.Arena.Parent.StaticRange = nil
		create.Arena.Parent.DynamicRange = 23
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(Format(m), "parent-range=dynamic#23") {
			t.Fatal("lost symbolic backing range")
		}
	}
}

// Nested metadata cannot silently drop or strengthen the retained parent
// dependency; malformed bounds and parent/child identity conflation are errors.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§50,97.
func TestNestedArenaDependencyContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Module, *Operation)
	}{
		{"missing dependency", func(m *Module, o *Operation) { o.Arena.Parent = nil }},
		{"missing storage", func(m *Module, o *Operation) { o.Arena.Parent.Storage = 0 }},
		{"different storage", func(m *Module, o *Operation) { o.Arena.Parent.Storage = 8 }},
		{"different parent epoch", func(m *Module, o *Operation) { o.Arena.Parent.Reference.EpochDependency++ }},
		{"different parent domain", func(m *Module, o *Operation) { o.Arena.Parent.Reference.InvalidationDomain++ }},
		{"different borrow", func(m *Module, o *Operation) { o.Arena.Parent.Reference.Borrow++ }},
		{"different lifetime", func(m *Module, o *Operation) { o.Arena.Parent.Reference.Lifetime++ }},
		{"parent is child", func(m *Module, o *Operation) { o.Backing.Domain = 9 }},
		{"unresolved range", func(m *Module, o *Operation) { o.Arena.Parent.StaticRange = nil }},
		{"two range forms", func(m *Module, o *Operation) { o.Arena.Parent.DynamicRange = 1 }},
		{"inverted range", func(m *Module, o *Operation) { o.Arena.Parent.StaticRange.Start = big.NewInt(4097) }},
		{"negative range", func(m *Module, o *Operation) { o.Arena.Parent.StaticRange.Start = big.NewInt(-1) }},
		{"missing endpoint", func(m *Module, o *Operation) { o.Arena.Parent.StaticRange.EndExclusive = nil }},
		{"individual parent reclamation", func(m *Module, o *Operation) { o.Backing.Reclamation = 3 }},
		{"metadata on terminal operation", func(m *Module, o *Operation) { m.Functions[0].Blocks[0].Operations[2].Arena.Parent = o.Arena.Parent }},
		{"invented parent for ordinary slice", func(m *Module, o *Operation) { p := m.Functions[0].Parameters[3].Reference; p.Provenance = "parameter" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, create := nestedArenaFixture(64)
			test.change(m, create)
			if err := Verify(m); err == nil {
				t.Fatal("malformed nested dependency accepted")
			}
		})
	}
}

// A live child blocks all terminal/reset causes on the parent, including a
// may-live child at a branch join. Child Reset retains the backing dependency.
// Rules: rules/memory/arena.md — §51(4)–(7).
func TestNestedArenaBlocksParentInvalidation(t *testing.T) {
	for _, kind := range []OpKind{OpArenaReset, OpArenaRelease, OpArenaDestroy} {
		m, _ := nestedArenaFixture(64)
		reset := nestedParentReset(m)
		reset.Kind = kind
		if kind != OpArenaReset {
			reset.Operands = []ValueID{0}
			reset.Arena.EpochAfter = 0
			reset.Backing.Transition = string(sema.BackingTransitionEndDomain)
			if kind == OpArenaDestroy {
				typ := m.Types.InternArena()
				m.DestructionPlans = []DestructionPlanRecord{{ID: "destroy-Arena", Type: typ, Kind: "compiler-known-resource", Resource: "Arena"}}
				reset.Ownership = &OwnershipFacts{DestructionPlan: "destroy-Arena", Cause: "scope-exit"}
			}
		}
		block := m.Functions[0].Blocks[0]
		block.Operations = append(block.Operations[:2], reset, block.Operations[2], Operation{Kind: OpReturn})
		if err := Verify(m); err == nil || !strings.Contains(err.Error(), "remains live") {
			t.Fatalf("%s bypassed child: %v", kind, err)
		}
	}
}

// Dependency state flows across CFG edges and converges through loops. A child
// released on every path is dead at the merge; one surviving path blocks Reset.
// Rules: rules/memory/arena.md — §51(5)–(7).
func TestNestedArenaDependencyCFG(t *testing.T) {
	for _, releaseBoth := range []bool{false, true} {
		m, _ := nestedArenaFixture(64)
		fn := m.Functions[0]
		entry := fn.Blocks[0]
		release := entry.Operations[2]
		reset := nestedParentReset(m)
		boolean := m.Types.Intern(Type{Kind: TypeBool})
		fn.Parameters = append(fn.Parameters, Parameter{Name: "flag", Value: Value{ID: 8, Type: boolean, Ownership: OwnershipImmediate}})
		entry.Operations = append(entry.Operations[:2], Operation{Kind: OpCondBranch, Operands: []ValueID{8}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}})
		left := &Block{ID: 1, Operations: []Operation{release, {Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}}
		right := &Block{ID: 2}
		if releaseBoth {
			right.Operations = append(right.Operations, release)
		}
		right.Operations = append(right.Operations, Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}})
		join := &Block{ID: 3, Operations: []Operation{reset, {Kind: OpReturn}}}
		fn.Blocks = append(fn.Blocks, left, right, join)
		err := Verify(m)
		if releaseBoth && err != nil {
			t.Fatal(err)
		}
		if !releaseBoth && (err == nil || !strings.Contains(err.Error(), "remains live")) {
			t.Fatalf("may-live child ignored: %v", err)
		}
	}
	// Releasing a child before a backedge permits the next dynamic creation.
	m, _ := nestedArenaFixture(64)
	fn := m.Functions[0]
	// Use only the liveness checker here: the slice parameter transfer itself
	// would need source-level reestablishment in each iteration (P19-20).
	block := fn.Blocks[0]
	block.Operations[len(block.Operations)-1] = Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 0}}}
	if err := verifyNestedArenaLiveness(fn, map[BlockID]*Block{0: block}); err != nil {
		t.Fatal(err)
	}
}

// A represented source range cannot be silently replaced with wider or freshly
// invented bounds. Unrepresented caller bounds remain explicit plan facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §97.
func TestNestedArenaPreservesKnownBounds(t *testing.T) {
	m, create := nestedArenaFixture(64)
	source := m.Functions[0].Parameters[3].Reference
	bounds := &SliceFacts{StaticRange: &StaticSliceRange{Start: big.NewInt(0), EndExclusive: big.NewInt(4096)}}
	if err := verifyArenaParent(*create, source, bounds); err != nil {
		t.Fatal(err)
	}
	create.Arena.Parent.StaticRange.EndExclusive = big.NewInt(8192)
	if err := verifyArenaParent(*create, source, bounds); err == nil {
		t.Fatal("widened backing range accepted")
	}
	create.Arena.Parent.StaticRange = nil
	create.Arena.Parent.DynamicRange = 3
	bounds.StaticRange = nil
	bounds.DynamicRange = 3
	if err := verifyArenaParent(*create, source, bounds); err != nil {
		t.Fatal(err)
	}
	create.Arena.Parent.DynamicRange = 4
	if err := verifyArenaParent(*create, source, bounds); err == nil {
		t.Fatal("changed range identity accepted")
	}
}

// Resetting the child, even one borrowing zero bytes, retains its parent
// dependency. Only terminal Release/Destroy ends that retained borrow.
// Rules: rules/memory/arena.md — §51(5)–(7).
func TestNestedArenaChildResetRetainsDependency(t *testing.T) {
	m, create := nestedArenaFixture(64)
	create.Arena.Parent.StaticRange.EndExclusive = big.NewInt(0)
	parent := nestedParentReset(m)
	child := parent
	facts := *parent.Arena
	backing := *parent.Backing
	facts.EpochBefore = 7
	facts.EpochAfter = 8
	backing.Domain = 5
	child.Arena = &facts
	child.Backing = &backing
	fn := m.Functions[0]
	entry := fn.Blocks[0]
	// Isolate the lifecycle transfer function; source receiver Places and actual
	// source plan construction remain P19-20 work.
	entry.Operations = []Operation{*create, child, parent, {Kind: OpReturn}}
	if err := verifyNestedArenaLiveness(fn, map[BlockID]*Block{0: entry}); err == nil || !strings.Contains(err.Error(), "remains live") {
		t.Fatalf("child Reset ended parent dependency: %v", err)
	}
}
