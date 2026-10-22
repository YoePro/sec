package semantic

import (
	"strings"
	"testing"
)

// package15MatrixFixture covers the P15-49 forms the other fixtures do not:
// parameter and local holders, a mutable holder move, shared holder copies,
// field/element sub-references, legal reborrows, reference equality,
// whole-referent writes with their evaluation order, compound writes through
// a reborrow and a Sema-placed borrow end.
const package15MatrixFixture = `module main

type Pair struct {
    left: int,
    right: int,
}

fn Compute() int {
    return 7
}

fn Store(target: ref mut int) void {
    *target = Compute()
}

fn Pass(pair: ref mut Pair) int {
    let moved :<- pair
    let left := ref mut moved.left
    *left += 1
    return moved.right
}

fn Shared(pair: ref Pair) bool {
    let copy := pair
    let right := ref copy.right
    return copy == pair && *right == pair.right
}

fn main() int {
    let mut pair := Pair { left: 1, right: 2 }
    let mut values := [1, 2, 3, 4]
    let element := ref mut values[1]
    *element = 5
    Store(pair.left)
    let total := Pass(pair)
    if Shared(ref pair) {
        return total + pair.right - 4
    }
    return 1
}
`

// P15-49: source-to-Semantic-IR facts of the reference matrix. The target
// Place of `*target = Compute()` is formed before the source is evaluated;
// moving a `ref mut` holder or copying a shared one reuses the holder's SSA
// value (no hidden copy of the referent); reborrows name their parent borrow;
// equality compares the reference SSA values; the element borrow ends at its
// last use, before the call that borrows the same aggregate again.
func TestPackage15SourceMatrix(t *testing.T) {
	module, err := buildPackage15Source(t, package15MatrixFixture, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	operations := func(name string) []Operation {
		var ops []Operation
		for _, block := range functionNamed(t, module, name).Blocks {
			ops = append(ops, block.Operations...)
		}
		return ops
	}
	index := func(ops []Operation, kind OpKind, from int) int {
		for i := from; i < len(ops); i++ {
			if ops[i].Kind == kind {
				return i
			}
		}
		return -1
	}

	store := operations("Store")
	deref, call, write := index(store, OpReferenceDerefPlace, 0), index(store, OpDirectCall, 0), index(store, OpPlaceWrite, 0)
	if !(deref >= 0 && deref < call && call < write) {
		t.Fatalf("Store must form the target Place before evaluating Compute():\n%s", Format(module))
	}

	pass := functionNamed(t, module, "Pass")
	parameter := pass.Parameters[0].Value.ID
	for _, op := range operations("Pass") {
		if op.Kind == OpReferenceMove || op.Kind == OpReferenceCopyShared {
			t.Fatalf("moving a ref mut holder must reuse its SSA value, not emit %s", op.Kind)
		}
		if op.Kind == OpReferenceReborrowMutable && (op.Reference == nil || op.Reference.SourceBorrow != pass.Parameters[0].Reference.Borrow) {
			t.Fatalf("the field reborrow must name the parameter borrow as its source: %+v", op.Reference)
		}
	}
	derefsOfParameter := 0
	for _, op := range operations("Pass") {
		if op.Kind == OpReferenceDerefPlace && op.Operands[0] == parameter {
			derefsOfParameter++
		}
	}
	if derefsOfParameter < 2 {
		t.Fatalf("both `moved` projections must dereference the parameter's own SSA value:\n%s", Format(module))
	}

	shared := functionNamed(t, module, "Shared")
	for _, op := range operations("Shared") {
		if op.Kind == OpReferenceCompare && (op.Operands[0] != shared.Parameters[0].Value.ID || op.Operands[1] != shared.Parameters[0].Value.ID) {
			t.Fatalf("a shared holder copy must compare the same reference SSA value: %+v", op)
		}
	}

	main := operations("main")
	end := index(main, OpReferenceEndBorrow, 0)
	storeCall := -1
	for i, op := range main {
		if op.Kind == OpDirectCall && strings.Contains(string(op.Callee), "Store") {
			storeCall = i
		}
	}
	if end < 0 || storeCall < 0 || end > storeCall {
		t.Fatalf("the element borrow must end at its last use, before Store(pair.left):\n%s", Format(module))
	}
}

// P15-49 invariants over every successful Package 15 fixture: modules
// verify, whole-value reads through references are never aggregate copies,
// and no undef/poison or hidden allocation appears.
func TestPackage15FixtureInvariants(t *testing.T) {
	for name, source := range package15Fixtures() {
		module, err := buildPackage15Source(t, source, "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: verify: %v", name, err)
		}
		text := Format(module)
		for _, forbidden := range []string{"undef", "poison", "alloc"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s: module contains %q", name, forbidden)
			}
		}
		for _, function := range module.Functions {
			for _, block := range function.Blocks {
				for _, op := range block.Operations {
					if op.Kind != OpPlaceRead || len(op.Results) != 1 {
						continue
					}
					typ, _ := module.Types.Lookup(op.Results[0].Type)
					if typ.Kind == TypeStruct || typ.Kind == TypeArray {
						t.Fatalf("%s: %s reads a whole aggregate through a reference", name, function.ID)
					}
				}
			}
		}
	}
}
