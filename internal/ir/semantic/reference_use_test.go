package semantic

import (
	"errors"
	"strings"
	"testing"

	"sec/internal/sema"
)

const referenceUseFixture = `module main

type Pair struct {
    left: int,
    right: int,
}

fn Use() int {
    let pair := Pair { left: 1, right: 2 }
    let p := ref pair
    let l := p.left
    let q := ref pair
    let same := p == q
    let mut state := Pair { left: 3, right: 4 }
    let w := ref mut state
    w.right = l + 1
    return l
}
`

// Reads, writes and equality through proven references consume the holder's
// reference SSA value, keep its proof metadata and emit no runtime check: a
// read is ref.deref-place + place.field + place.read, a write forms the
// destination Place before evaluating the source and ends in place.write,
// and equality is ref.compare. No aggregate is loaded or copied (P15-46).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 44-52
//   - rules/foundations/operators.md — "Assignment evaluation order"
func TestPackage15LowersProvenReferenceUses(t *testing.T) {
	module, err := analyzedModule(t, referenceUseFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"ref.deref-place %4 {borrow=#1", "place.read", "ref.compare %4, %9 eq", "place.write"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"ref.is-valid", "fail.reference-generation", "struct.extract-field", "storage.load"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("proven reference use emitted %q:\n%s", forbidden, text)
		}
	}
	use := functionNamed(t, module, "Use")
	var order []OpKind
	for _, block := range use.Blocks {
		for _, op := range block.Operations {
			switch op.Kind {
			case OpPlaceField, OpIntBinaryChecked, OpPlaceWrite:
				order = append(order, op.Kind)
			}
		}
	}
	// read field, destination field, source arithmetic, write
	want := []OpKind{OpPlaceField, OpPlaceField, OpIntBinaryChecked, OpPlaceWrite}
	if len(order) != len(want) {
		t.Fatalf("operation order = %v, want %v", order, want)
	}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("operation order = %v, want destination Place before source", order)
		}
	}
}

// A dynamic-epoch reference is validated once on the same reference SSA; the
// false edge terminates in fail.reference-generation and the dereference
// exists only on the true edge, carrying the guard.
func TestPackage15DynamicReferenceUseValidatesOnce(t *testing.T) {
	module, err := analyzedModule(t, referenceUseFixture, 15)
	if err != nil {
		t.Fatal(err)
	}
	var referenceType, pairType, placeType TypeID
	for id := TypeID(1); ; id++ {
		typ, ok := module.Types.Lookup(id)
		if !ok {
			break
		}
		if typ.Kind == TypeReference && !typ.Mutable {
			referenceType, pairType = id, typ.Element
		}
	}
	placeType = module.Types.Intern(Type{Kind: TypePlace, Element: pairType})
	function := &Function{ID: "main::dynamic(ref Pair)", Name: "dynamic", ReturnType: pairType}
	metadata := &ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: ReferenceDynamicEpoch, InvalidationDomain: 1, EpochDependency: 1, AddressSpace: AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
	// A dynamic-epoch reference parameter carries its facts, so the verifier
	// requires a guard for every dereference of it (P15-47).
	parameter := Value{ID: 1, Type: referenceType, Ownership: OwnershipSharedReference}
	function.Parameters = []Parameter{{Name: "r", Value: parameter, Reference: metadata}}
	module.Functions = append(module.Functions, function)
	owner := &builder{module: module, maxPackage: 15}
	fb := &functionBuilder{owner: owner, fn: function, nextValue: 2, bindings: map[sema.BindingID]binding{}}
	fb.current = fb.newBlock()
	function.Entry = fb.current.ID
	reference := builtValue{id: parameter.ID, typ: referenceType}
	guard, err := fb.validateReference(reference, metadata, Location{})
	if err != nil || guard == 0 {
		t.Fatalf("validate = %d, %v; want a dynamic guard", guard, err)
	}
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{reference.id}, Reference: metadata, ReferenceGuard: guard}, placeType)
	value := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}}, pairType)
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{value.id}})
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	checks := 0
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReferenceIsValid {
				checks++
			}
		}
	}
	if checks != 1 {
		t.Fatalf("want exactly one validity check, got %d:\n%s", checks, Format(module))
	}

	// Without the guard the same dereference is rejected by the verifier.
	for _, block := range function.Blocks {
		for index := range block.Operations {
			if block.Operations[index].Kind == OpReferenceDerefPlace {
				block.Operations[index].ReferenceGuard = 0
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "requires its ref.is-valid guard") {
		t.Fatalf("unguarded dynamic dereference verified: %v", err)
	}
}

// An explicit `ref r` over a reference holder has no decided meaning
// (MD-061) and stays a package-15 boundary.
func TestPackage15ExplicitReborrowAwaitsDecision(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Use() int {
    let mut value := 1
    let first := ref mut value
    let again := ref first
    return 0
}
`, 15)
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "reborrow of reference") {
		t.Fatalf("module=%t err=%v, want the reborrow boundary", module != nil, err)
	}
}
