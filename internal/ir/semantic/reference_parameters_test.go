package semantic

import (
	"strings"
	"testing"
)

const referenceParameterFixture = `module main

type Pair struct {
    left: int,
    right: int,
}

fn Peek(pair: ref Pair) int {
    return pair.left
}

fn Bump(pair: ref mut Pair) void {
    pair.right = 3
}

fn Use() int {
    let pair := Pair { left: 1, right: 2 }
    let a := Peek(pair)
    let p := ref pair
    let b := Peek(p)
    let mut state := Pair { left: 1, right: 2 }
    Bump(state)
    let w := ref mut state
    Bump(w)
    return a + b
}
`

// Reference parameters carry their callee-side facts (caller-owned borrow),
// reads and writes through them use the P15-46 policy, an owned Place passed
// to a ref/ref mut parameter becomes Sema's implicit borrow, an existing
// shared reference is copied with its borrow, and an existing ref mut is
// reborrowed for the call instead of being silently copied (P15-47).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 59-60
//   - rules/memory/borrowing.md — § 11 "Reference values and reborrowing"
func TestPackage15LowersReferenceParametersAndPassing(t *testing.T) {
	module, err := analyzedModule(t, referenceParameterFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		`Peek(%0 pair: !3 [shared-reference] {borrow=#1 lifetime=#1 validity=proven proof=caller-owned-borrow`,
		`[mutable-reference] {borrow=#3 lifetime=#3 validity=proven proof=caller-owned-borrow`,
		"ref.deref-place %0", "place.read", "place.write",
		`call.direct "main::Peek(ref Pair)"(%4[shared-reference])`,
		"ref.copy-shared %7", "ref.reborrow-mut %16",
		`call.direct "main::Bump(ref mut Pair)"(%17[mutable-reference])`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	use := functionNamed(t, module, "Use")
	borrows := 0
	for _, block := range use.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReferenceBorrowShared || op.Kind == OpReferenceBorrowMutable {
				borrows++
			}
			if op.Kind == OpReferenceCopyShared {
				if typ, _ := module.Types.Lookup(op.Results[0].Type); typ.Mutable {
					t.Fatalf("ref mut was copied:\n%s", text)
				}
			}
		}
	}
	if borrows != 4 {
		t.Errorf("want two implicit and two explicit borrows, got %d:\n%s", borrows, text)
	}

	// A reference argument must carry its reference action.
	for _, block := range use.Blocks {
		for index := range block.Operations {
			if block.Operations[index].Kind == OpDirectCall {
				block.Operations[index].ArgumentActions[0] = ArgumentCopyTrivial
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), `want "shared-reference"`) {
		t.Fatalf("verify accepted a copied reference argument: %v", err)
	}
}
