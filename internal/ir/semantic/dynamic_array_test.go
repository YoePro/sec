package semantic

import (
	"strings"
	"testing"
)

// Package 18 owning dynamic arrays in Semantic IR: `T[]` is its own type
// (not a fixed array or slice), move-only with a non-trivial destruction
// plan; the empty owner is dynamic-array.empty (no allocation, no element);
// `.Len` and `len(...)` are the same uint count observed through the
// owner's Place without moving it; an owned value is tracked for cleanup
// and moves into a by-value parameter. Package 17 still rejects the type;
// a non-empty owning literal is a Sec 0.1 Sema error, not an IR boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 5, 7, 13, 49, 50, 52–54, 80, 83, 85.
func TestDynamicArraySemanticIR(t *testing.T) {
	source := `module main

fn Count(values: int[]) uint {
    return values.Len + len(values)
}

fn Empty() uint {
    let mut values: int[]
    let other: int[] := []
    let first := values.Len + other.Len
    return first + Count(<-other)
}

fn main() int {
    return 0
}
`
	module, err := analyzedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"dynamic-array<!", "dynamic-array.empty", "dynamic-array.length", "place.value-root", "place.storage-root",
		`dynamic-array element="destroy::int:int"`, "cleanup.track-owned",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "array<!2, ") {
		t.Errorf("a dynamic array must not be a fixed array:\n%s", text)
	}
	if strings.Count(text, "dynamic-array.empty") != 2 {
		t.Errorf("want one empty owner per declaration:\n%s", text)
	}

	for _, item := range []struct {
		source  string
		pkg     uint8
		feature string
	}{
		{"fn F(values: int[]) uint {\n    return values.Len\n}\n", 17, "dynamic array type"},
	} {
		_, err := analyzedModule(t, "module main\n\n"+item.source+"\nfn main() int {\n    return 0\n}\n", item.pkg)
		if err == nil || !strings.Contains(err.Error(), item.feature) {
			t.Errorf("package %d %q: want %q, got %v", item.pkg, item.source, item.feature, err)
		}
	}
}

// The dynamic-array type and operations verify only in their canonical
// shapes: no length type parameter, an owned empty result, and length over
// a Place of a dynamic array.
func TestDynamicArrayVerifierShapes(t *testing.T) {
	table := NewTypeTable()
	intType := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	if _, err := table.InternDynamicArray(intType); err != nil {
		t.Fatal(err)
	}
	if err := verifyDynamicArrayType(table, 1, Type{Kind: TypeDynamicArray, Element: intType, Length: "3"}); err == nil {
		t.Error("a dynamic array accepted a length type parameter")
	}
	array, _ := table.InternDynamicArray(intType)
	uintType := table.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	values := map[ValueID]Value{1: {ID: 1, Type: array, Ownership: OwnershipOwned}}
	if err := verifyDynamicArrayOperation(table, Operation{Kind: OpDynamicArrayEmpty, Results: []Value{{ID: 2, Type: array, Ownership: OwnershipImmediate}}}, values); err == nil {
		t.Error("an empty owner must be owned")
	}
	if err := verifyDynamicArrayOperation(table, Operation{Kind: OpDynamicArrayLength, Operands: []ValueID{1}, Results: []Value{{ID: 3, Type: uintType}}}, values); err == nil {
		t.Error("length must observe a Place, never move the owner")
	}
}

// Owning-array element access forms the owner Place first, evaluates the
// index once, branches on dynamic-array.index-in-bounds and forms the
// element Place only on the success edge; the Place keeps the owner's
// authority and names its backing domain. Reads, plain and compound writes
// use place.read/place.write; a try index routes IndexError; a direct
// element borrow borrows that guarded Place. A path through an element
// stays a named boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 40–43, 74–79.
func TestDynamicArrayElementPlaces(t *testing.T) {
	source := `module main

fn Read(values: int[], i: int) int {
    return values[i]
}

fn Write(i: uint) void {
    let mut values: int[]
    values[i] = 3
    values[i] += 1
}

fn Try(values: int[]) Result[int, IndexError] {
    let v := try values[2]
    return Ok(v)
}

fn Borrow(i: uint) int {
    let mut values: int[]
    let r := ref mut values[i]
    *r = 4
    let s := ref values[i]
    return *s
}

fn main() int {
    return 0
}
`
	module, err := analyzedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"dynamic-array.index-in-bounds %2, %1 signed=true",
		"dynamic-array.element-place %2, %1 signed=true bounds=runtime-check proof=guarded guard=%3 {root=#1 storage=#1 backing=#1} : !5",
		"!5 = place<!1, ro>", "!9 = place<!1, rw>", "place.write %5, %6", "int.binary-checked add",
		`fail.bounds operation="dynamic-array-index"`, "index=2", "result.err", "cleanup.run-function exit=error-propagation",
		"ref.borrow-mut", "ref.borrow-shared",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "own.move-from-place") {
		t.Errorf("an element access must not move the owner:\n%s", text)
	}
	for _, item := range []struct{ source, feature string }{
		{"type P struct {\n    x: int\n}\n\nfn F(values: P[]) int {\n    let r := ref values[0].x\n    return *r\n}\n", "Place path through a dynamic-array element"},
	} {
		_, err := analyzedModule(t, "module main\n\n"+item.source+"\nfn main() int {\n    return 0\n}\n", 18)
		if err == nil || !strings.Contains(err.Error(), item.feature) {
			t.Errorf("%q: want %q, got %v", item.source, item.feature, err)
		}
	}
}

// An element Place keeps the owner's authority, has a runtime guard (T[]
// has no length proof) and names the backing domain it depends on.
func TestDynamicArrayElementPlaceVerifier(t *testing.T) {
	table := NewTypeTable()
	intType := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	array, _ := table.InternDynamicArray(intType)
	owner := table.Intern(Type{Kind: TypePlace, Element: array})
	element := table.Intern(Type{Kind: TypePlace, Element: intType})
	writable := table.Intern(Type{Kind: TypePlace, Element: intType, Mutable: true})
	values := map[ValueID]Value{1: {ID: 1, Type: owner}, 2: {ID: 2, Type: intType}}
	valid := Operation{
		Kind: OpDynamicArrayElementPlace, Operands: []ValueID{1, 2}, Results: []Value{{ID: 4, Type: element}},
		ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: 3,
		Place: &PlaceMetadata{Root: 1, BackingDomain: 7},
	}
	if err := verifyDynamicArrayOperation(table, valid, values); err != nil {
		t.Fatalf("canonical element Place rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Operation){
		"authority":  func(op *Operation) { op.Results = []Value{{ID: 4, Type: writable}} },
		"no guard":   func(op *Operation) { op.ArrayGuard = 0 },
		"proven":     func(op *Operation) { op.ArrayCheckKind, op.ArrayProofKind, op.ArrayGuard = ArrayIndexProvenSafe, ArrayIndexProofConstant, 0 },
		"no backing": func(op *Operation) { op.Place = &PlaceMetadata{Root: 1} },
	} {
		op := valid
		mutate(&op)
		if err := verifyDynamicArrayOperation(table, op, values); err == nil {
			t.Errorf("%s: malformed element Place accepted", name)
		}
	}
}

// Slicing an owner borrows its Place with the runtime length as extent:
// an open range needs no check, an explicit range is checked against that
// same length; an empty owner yields a valid empty slice without backing,
// and indexing it fails through the ordinary bounds path.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 13, 78–80.
func TestDynamicArrayOwnerSlicing(t *testing.T) {
	source := `module main

fn All(values: int[]) uint {
    let s := ref values[..]
    return s.Len
}

fn Range(a: uint, b: uint) uint {
    let mut values: int[]
    let s := ref mut values[a..b]
    return s.Len
}

fn main() int {
    return 0
}
`
	module, err := analyzedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"%2 = dynamic-array.length %1", "slice.range-normalize %2", "proof=source-open-bound", "slice.borrow-shared %1, %3, %4",
		"dynamic-array.empty", "%5 = dynamic-array.length %4", "slice.range-check %5", `fail.bounds operation="slice-range"`, "slice.borrow-mut %4, %6, %7",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "const.int") && strings.Contains(text, "slice.range-normalize %2") && strings.Contains(text, "%2 = const.int") {
		t.Errorf("an owner's extent must be its runtime length, not a constant:\n%s", text)
	}
}
