package semantic

import (
	"errors"
	"strings"
	"testing"
)

// At package 15 every reference and Place path is routed through the
// compiler-resolved P15 plans: a reference creation, implicit call-site
// borrow, reference parameter or reference use without its lowering is a
// package-15 unsupported error naming the planned facts, the build returns no
// module, and programs without references build exactly as at package 14
// (P15-41).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 33-36, 63
func TestPackage15BuilderConsumesReferencePlans(t *testing.T) {
	plain := `module main

fn Add(left: int, right: int) int {
    return left + right
}
`
	at14, err14 := analyzedModule(t, plain, 14)
	at15, err15 := analyzedModule(t, plain, 15)
	if err14 != nil || err15 != nil || Format(at14) != Format(at15) {
		t.Fatalf("reference-free program differs between packages 14 and 15: %v / %v", err14, err15)
	}

	for name, test := range map[string]struct{ source, want string }{
		"reborrow pending reference lowering": {`module main

fn Use() int {
    let mut value := 1
    let first := ref mut value
    let again := ref first
    return 0
}
`, "reborrow of reference shared-direct of local storage"},
		"reference parameter": {`module main

type Pair struct {
    left: int,
    right: int,
}

fn Peek(pair: ref Pair) int {
    return 0
}

fn Use() int {
    let pair := Pair { left: 1, right: 2 }
    return Peek(pair)
}
`, "reference parameter pair"},
	} {
		module, err := analyzedModule(t, test.source, 15)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Package != 15 || !strings.Contains(unsupported.Feature, test.want) {
			t.Errorf("%s: module=%v err=%v, want package-15 unsupported %q", name, module != nil, err, test.want)
		}
	}
}

// Below package 15 the established boundaries are unchanged: reference types
// remain unsupported types and the P14 fixed-array element borrow diagnostic
// is kept.
func TestPackage15BoundaryLeavesEarlierPackagesUnchanged(t *testing.T) {
	_, err := analyzedModule(t, `module main

fn Use(values: int[3]) int {
    let element := ref values[1]
    discard element
    return 0
}
`, 14)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 14 || !strings.Contains(unsupported.Feature, "fixed-array element borrow") {
		t.Fatalf("package 14 error = %v", err)
	}
}

// Borrows of whole local bindings lower through Place roots: an immutable
// binding becomes a read-only place.value-root over its SSA value, a mutable
// binding a place.storage-root over its P5 storage, which is marked
// address-taken; the borrow carries the resolved reference facts unchanged
// and the module verifies. Static roots stay unsupported (P15-42).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 64-67
func TestPackage15LowersLocalRootBorrows(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Use() int {
    let value := 1
    let shared := ref value
    let mut counter := 2
    let exclusive := ref mut counter
    return value
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		"place.value-root %0 {root=#",
		"kind=local",
		"ref.borrow-shared %",
		"validity=proven proof=lexical-borrow",
		"place.storage-root $1 {root=#",
		"ref.borrow-mut %",
		"address-taken",
		"= place<!1, ro>",
		"= place<!1, rw>",
		"= ref<!1>",
		"= ref_mut<!1>",
		"[compiler-temporary]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "storage.load") {
		t.Errorf("borrowing a local must not load or copy it:\n%s", text)
	}

	_, err = analyzedModule(t, `module main

let shared: int := 7

fn Use() int {
    let fixed := ref shared
    return 0
}
`, 15)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 15 || !strings.Contains(unsupported.Feature, "static Place root") {
		t.Fatalf("static root error = %v", err)
	}
}

// Field and nested subfield borrows lower through place.field projections
// carrying the P13 field identity from the resolved member plan; the struct
// value is neither loaded nor copied, authority follows the root (immutable
// SSA root ro, mutable storage root rw), and a mutable field borrow needs a
// read/write root (P15-43).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 39, 43, 68
func TestPackage15LowersFieldBorrowsWithoutAggregateCopies(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Inner struct {
    count: int,
    limit: int,
}

type Outer struct {
    inner: Inner,
    flag: bool,
}

fn Use() int {
    let outer := Outer { inner: Inner { count: 1, limit: 2 }, flag: true }
    let count := ref outer.inner.count
    let mut state := Outer { inner: Inner { count: 3, limit: 4 }, flag: false }
    let limit := ref mut state.inner.limit
    return 0
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"place.value-root", "place.field %", "field=#0", "field=#1", "place.storage-root $1", "ref.borrow-shared", "ref.borrow-mut", "address-taken"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"struct.extract-field", "storage.load"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("field borrow copied or loaded its aggregate (%s):\n%s", forbidden, text)
		}
	}
	if strings.Count(text, "place.field %") != 4 {
		t.Errorf("want two field projections per borrow:\n%s", text)
	}
}

// A fixed-array element borrow forms the root array Place, evaluates its index
// once after the root, and consumes P14's resolved bounds plan: a proven index
// forms the element Place directly and a runtime-checked index emits exactly
// one Place bounds predicate and branches to the ordinary bounds failure before
// the element Place and reference exist. No element or array is copied
// (P15-44).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 39-43, 68-70
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — §§ 32-36
func TestPackage15LowersFixedArrayElementBorrows(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Use(values: int[3], at: int) int {
    let first := ref values[1]
    let mut grid := [4, 5, 6]
    let picked := ref mut grid[at]
    return 0
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"place.value-root", "place.storage-root", "place.array-element", "place.array-index-in-bounds", "fail.bounds operation=\"fixed-array-index\"", "ref.borrow-shared", "ref.borrow-mut"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"array.extract", "storage.load"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("element borrow copied or loaded its array (%s):\n%s", forbidden, text)
		}
	}
	if got := strings.Count(text, "place.array-index-in-bounds"); got != 1 {
		t.Errorf("want one bounds predicate for the dynamic borrow only, got %d:\n%s", got, text)
	}
	if got := strings.Count(text, "place.array-element"); got != 2 {
		t.Errorf("want two element Places, got %d:\n%s", got, text)
	}
}
