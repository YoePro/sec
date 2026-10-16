package semantic

import (
	"strings"
	"testing"
)

const placeBorrowIntegrationFixture = `module main

type Cell struct {
    v: int,
    w: int,
}

type Inner struct {
    count: Cell,
    limit: Cell,
}

type Wide struct {
    small: int128,
    large: uint256,
}

type Outer struct {
    inner: Inner,
    wide: Wide,
}

fn Disjoint() int {
    let mut outer := Outer { inner: Inner { count: Cell { v: 1, w: 1 }, limit: Cell { v: 2, w: 2 } }, wide: Wide { small: 3, large: 4 } }
    let count := ref mut outer.inner.count
    let limit := ref mut outer.inner.limit
    count.v = 5
    limit.v = 6
    return 0
}

fn WideFields() int {
    let mut outer := Outer { inner: Inner { count: Cell { v: 1, w: 1 }, limit: Cell { v: 2, w: 2 } }, wide: Wide { small: 3, large: 4 } }
    let wide := ref mut outer.wide
    wide.large = 9
    let small := wide.small
    return 0
}

fn Nested(outer: ref mut Outer) int {
    let count := ref mut outer.inner.count
    count.v = 7
    let limit := ref outer.inner.limit
    return limit.v
}

fn Elements() int {
    let mut grid := [Cell { v: 1, w: 1 }, Cell { v: 2, w: 2 }, Cell { v: 3, w: 3 }]
    let first := ref mut grid[0]
    let second := ref mut grid[1]
    first.v = 4
    second.v = 5
    return 0
}

fn WideIndex(at: int128, big: uint256) int {
    let values := [Cell { v: 1, w: 1 }, Cell { v: 2, w: 2 }, Cell { v: 3, w: 3 }]
    let a := ref values[at]
    let b := ref values[big]
    return a.v + b.v
}

fn ZeroLength(at: int) int {
    let empty: Cell[0] := []
    let cell := ref empty[at]
    return cell.v
}
`

// P13/P14 integration of field and element borrows (P15-61, P15-62): two
// disjoint mutable field borrows and two distinct constant element borrows
// coexist; int128/uint256 field referents are borrowed without copying their
// aggregate; a nested sub-field reborrow through a ref mut parameter
// dereferences the holder once, projects the fields and derives the new
// borrow from that sub-Place with the parameter's facts; int128 and uint256
// dynamic indexes are each bounds-checked exactly once; and a zero-length
// array has no proven-valid element, so its element borrow is runtime-checked.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 43, 46-48, 78, 147, 148
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — §§ 86, 96, 97
func TestPackage15FieldAndElementBorrowIntegration(t *testing.T) {
	module, err := analyzedModule(t, placeBorrowIntegrationFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	count := func(fn *Function, kinds ...OpKind) int {
		total := 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				for _, kind := range kinds {
					if op.Kind == kind {
						total++
					}
				}
			}
		}
		return total
	}
	for _, name := range []string{"Disjoint", "WideFields", "Nested", "Elements", "WideIndex", "ZeroLength"} {
		fn := functionNamed(t, module, name)
		if copies := count(fn, OpStructExtractField, OpArrayExtract, OpStorageLoad); copies != 0 {
			t.Errorf("%s copied or loaded an aggregate to form a reference (%d operations)\n%s", name, copies, text)
		}
	}
	if got := count(functionNamed(t, module, "Disjoint"), OpReferenceBorrowMutable); got != 2 {
		t.Errorf("Disjoint: want two mutable field borrows, got %d", got)
	}
	if got := count(functionNamed(t, module, "Elements"), OpReferenceBorrowMutable); got != 2 {
		t.Errorf("Elements: want two mutable element borrows, got %d", got)
	}
	if got := count(functionNamed(t, module, "Elements"), OpPlaceArrayIndexInBounds); got != 0 {
		t.Errorf("Elements: constant element borrows need no bounds branch, got %d", got)
	}
	if got := count(functionNamed(t, module, "WideIndex"), OpPlaceArrayIndexInBounds); got != 2 {
		t.Errorf("WideIndex: want one bounds check per dynamic index, got %d", got)
	}

	nested := functionNamed(t, module, "Nested")
	parameter := nested.Parameters[0].Reference
	reborrows := 0
	for _, block := range nested.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpReferenceReborrowMutable && op.Kind != OpReferenceReborrowShared {
				continue
			}
			reborrows++
			if op.Reference.SourceBorrow != parameter.Borrow || op.Reference.Provenance != "parameter" || op.Reference.ProofProvenance != "caller-owned-borrow" {
				t.Errorf("Nested: sub-field reborrow facts %+v do not derive from the parameter %+v", op.Reference, parameter)
			}
		}
	}
	if reborrows != 2 || count(nested, OpReferenceBorrowMutable, OpReferenceBorrowShared) != 0 {
		t.Errorf("Nested: want two sub-field reborrows and no fresh borrow\n%s", text)
	}

	zero := functionNamed(t, module, "ZeroLength")
	for _, block := range zero.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpPlaceArrayElement && op.ArrayCheckKind != "runtime-check" {
				t.Errorf("ZeroLength: a zero-length element borrow cannot be proven safe: %+v", op)
			}
		}
	}
	if count(zero, OpPlaceArrayIndexInBounds) != 1 || count(zero, OpBoundsFailure) != 1 {
		t.Errorf("ZeroLength: want one runtime bounds check ending in fail.bounds\n%s", text)
	}
	if !strings.Contains(text, "int128") && !strings.Contains(text, "uint256") {
		t.Errorf("wide referent types are missing:\n%s", text)
	}
}

// A sub-Place reborrow must derive from the dereference of the reference its
// SourceBorrow names, and a mutable reborrow needs a read/write sub-Place.
func TestPackage15VerifierChecksSubplaceReborrows(t *testing.T) {
	for name, mutate := range map[string]func(fn *Function, op *Operation){
		"foreign source borrow": func(fn *Function, op *Operation) { op.Reference.SourceBorrow++ },
		"owned Place source": func(fn *Function, op *Operation) {
			for _, block := range fn.Blocks {
				for index := range block.Operations {
					if deref := &block.Operations[index]; deref.Kind == OpReferenceDerefPlace && deref.Results[0].ID == firstDerefOf(fn, op.Operands[0]) {
						deref.Kind = OpPlaceValueRoot
						deref.Place = &PlaceMetadata{Root: 1, RootKind: PlaceRootParameter, Storage: 1, AddressSpace: AddressSpaceOrdinary}
						return
					}
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, placeBorrowIntegrationFixture, 15)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			fn := functionNamed(t, module, "Nested")
			mutated := false
			for _, block := range fn.Blocks {
				for index := range block.Operations {
					if op := &block.Operations[index]; op.Kind == OpReferenceReborrowMutable && !mutated {
						mutate(fn, op)
						mutated = true
					}
				}
			}
			if !mutated {
				t.Fatal("fixture has no sub-Place reborrow")
			}
			if err := Verify(module); err == nil {
				t.Fatalf("verifier accepted %s:\n%s", name, Format(module))
			}
		})
	}
}

// firstDerefOf follows Place projections from place back to the
// ref.deref-place result that roots them.
func firstDerefOf(fn *Function, place ValueID) ValueID {
	definitions := map[ValueID]Operation{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op
			}
		}
	}
	for {
		op, ok := definitions[place]
		if !ok {
			return 0
		}
		if op.Kind == OpReferenceDerefPlace {
			return place
		}
		place = op.Operands[0]
	}
}
