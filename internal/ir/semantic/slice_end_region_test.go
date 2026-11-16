package semantic

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/sema"
)

// sliceEndJoin builds a function whose shared slice parameter is ended on
// one branch and still live on the other when both reach a join whose Sema
// end candidate follows a real instruction (and, with useFirst, a use of the
// slice). No source program reaches this layout today (P16 closure).
func sliceEndJoin(t *testing.T, useFirst bool) (*Module, *functionBuilder) {
	t.Helper()
	module, err := analyzedModule(t, "module main\n\nfn main() int {\n    return 0\n}\n", 17)
	if err != nil {
		t.Fatal(err)
	}
	intType := module.Types.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	uintType := module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	boolType := module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	sliceType, err := module.Types.InternSlice(intType, false)
	if err != nil {
		t.Fatal(err)
	}
	facts := &ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: ReferenceProven, ProofProvenance: "caller-owned-borrow", AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "parameter"}
	function := &Function{ID: "main::joined(ref int[],bool)", Name: "joined", ReturnType: uintType}
	view := Value{ID: 1, Type: sliceType, Ownership: OwnershipSharedReference}
	flag := Value{ID: 2, Type: boolType, Ownership: OwnershipImmediate}
	function.Parameters = []Parameter{{Name: "view", Value: view, Reference: facts}, {Name: "flag", Value: flag}}
	module.Functions = append(module.Functions, function)
	fb := &functionBuilder{owner: &builder{module: module, maxPackage: 17}, fn: function, nextValue: 3, bindings: map[sema.BindingID]binding{}}
	fb.references = map[ValueID]*ReferenceMetadata{view.ID: facts}
	entry := fb.newBlock()
	function.Entry = entry.ID
	ended, live, join := fb.newBlock(), fb.newBlock(), fb.newBlock()
	fb.current = entry
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{flag.ID}, Successors: []BranchTarget{{Block: ended.ID}, {Block: live.ID}}})
	fb.current = ended
	fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{view.ID}, Reference: cloneReferenceMetadata(facts), Synthesized: true})
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID}}})
	fb.current = live
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID}}})
	fb.current = join
	result := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1)}, uintType)
	if useFirst {
		result = fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{view.ID}}, uintType)
	}
	fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{view.ID}, Reference: cloneReferenceMetadata(facts), Synthesized: true})
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{result.id}})
	return module, fb
}

// MD-065 § 6(3): a Sema end candidate at a mixed join that follows real
// instructions moves to the still-live incoming edge when nothing between
// sees the borrow; a use before the candidate keeps the fail-closed boundary
// of § 6(5).
// Rules: rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§ 6(3)–(5), 7(1).
func TestSliceBorrowEndMovesOutOfMixedJoin(t *testing.T) {
	module, fb := sliceEndJoin(t, false)
	if err := fb.normalizeSliceBorrowEnds(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if got := strings.Count(text, "ref.end-borrow %1"); got != 2 {
		t.Errorf("want the ended branch's end and one end on the live edge, got %d:\n%s", got, text)
	}

	_, blocked := sliceEndJoin(t, true)
	if err := blocked.normalizeSliceBorrowEnds(); err == nil || !strings.Contains(err.Error(), "resolved edge schedule") {
		t.Errorf("a use before the join's end must keep the boundary: %v", err)
	}
}

// Discarding a slice holder ends the authority it alone holds only after its
// derived reborrows have ended (borrowing.md §§ 8.2(1), 35.2(3)): a child,
// empty or not, used after `discard parent` keeps the parent's borrow, so no
// end marker is placed at the discard; without a child the discard ends it.
func TestDiscardedSliceParentOutlivedByChild(t *testing.T) {
	for name, item := range map[string]struct {
		source string
		ends   int
	}{
		"empty child":     {"fn F(values: int[4]) uint {\n    let parent := ref values[0..2]\n    let child := ref parent[1..<1]\n    discard parent\n    return child.Len\n}\n", 0},
		"non-empty child": {"fn F(values: int[4]) uint {\n    let parent := ref values[0..2]\n    let child := ref parent[1..1]\n    discard parent\n    return child.Len\n}\n", 0},
		"no child":        {"fn F(values: int[4]) uint {\n    let parent := ref values[0..2]\n    let n := parent.Len\n    discard parent\n    return n\n}\n", 1},
	} {
		module, err := analyzedModule(t, "module main\n\n"+item.source+"\nfn main() int {\n    return 0\n}\n", 17)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := strings.Count(Format(module), "ref.end-borrow"); got != item.ends {
			t.Errorf("%s: %d end markers, want %d:\n%s", name, got, item.ends, Format(module))
		}
	}
}
