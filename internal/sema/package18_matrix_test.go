package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// package18Case is one P18 matrix row: a source program under an allocation
// profile, and either no diagnostic (want == "") or a diagnostic containing
// want.
type package18Case struct {
	name, profile, source, want string
}

func runPackage18Cases(t *testing.T, cases []package18Case) {
	t.Helper()
	for _, check := range cases {
		t.Run(check.name, func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile("module main\n\n"+check.source+"\n", "p18.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := NewAnalyzer()
			profile := check.profile
			if profile == "" {
				profile = "hosted"
			}
			analyzer.SetAllocationProfile(profile)
			errors := analyzer.Analyze(parsed.Program)
			if check.want == "" {
				if len(errors) != 0 {
					t.Fatalf("unexpected diagnostics: %v", errors)
				}
				return
			}
			for _, err := range errors {
				if strings.Contains(err.Message, check.want) {
					return
				}
			}
			t.Fatalf("want a diagnostic containing %q, got %v", check.want, errors)
		})
	}
}

// P18-34: the § 119 Sema/type matrix. T[] is its own type, distinct from T[N]
// and ref T[]; its runtime length is not part of its identity; it is MoveOnly
// regardless of the element and never trivially destructible; a mutable
// binding without an initializer is the empty owner (also for an element
// type without a default, and without allocation), an immutable one is
// rejected; a struct containing T[] becomes move-only.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — § 119
//   - rules/collections/collections.md — dynamic arrays; rules/memory/copy_move.md — § 4
func TestPackage18SemaTypeMatrix(t *testing.T) {
	element := Type{Name: "int", Kind: IntType}
	array := NewDynamicArrayType(element)
	if TriviallyDestructible(array) {
		t.Error("T[] must be non-trivially destructible")
	}
	if got := CopyClassificationOf(array); got != CopyMoveOnly {
		t.Errorf("T[] of a copyable element is %s, want MoveOnly", got)
	}
	runPackage18Cases(t, []package18Case{
		{name: "distinct from T[N]", source: "fn F() void {\n    let mut values: int[]\n    let fixed: int[3] <- values\n    discard fixed\n}", want: "int[3]"},
		{name: "distinct from ref T[]", source: "fn Take(values: int[]) void {\n    discard values\n}\n\nfn F() void {\n    let mut values: int[]\n    Take(ref values)\n    discard values\n}", want: "Take"},
		{name: "runtime length not in identity", source: "fn Take(-> values: int[]) uint {\n    return values.Len\n}\n\nfn F() Result[uint, CollectionError] {\n    let mut one: int[]\n    try one.Append(1)\n    let mut two: int[]\n    try two.Append(1)\n    try two.Append(2)\n    return Ok(Take(<-one) + Take(<-two))\n}"},
		{name: "MoveOnly regardless of element", source: "fn F() void {\n    let mut values: int[]\n    let copied := values\n    discard copied\n    discard values\n}", want: "move-only"},
		{name: "mutable omitted initializer is empty", source: "fn F() uint {\n    let mut values: int[]\n    return values.Len\n}"},
		{name: "immutable omitted initializer rejected", source: "fn F() void {\n    let values: int[]\n    discard values\n}", want: "requires an initializer"},
		{name: "empty default independent of element defaultability", source: "type Handle struct {\n    id: int,\n}\n\nimpl Handle {\n    free {\n    }\n}\n\nfn F() uint {\n    let mut handles: Handle[]\n    return handles.Len\n}"},
		{name: "no hidden allocation in empty default", profile: "noalloc", source: "fn F() uint {\n    let mut values: int[]\n    return values.Len\n}\n\nfn main() int {\n    discard F()\n    return 0\n}"},
		{name: "struct containing T[] is move-only", source: "type Bag struct {\n    items: int[],\n}\n\nfn F(bag: Bag) Bag {\n    let copied := bag\n    discard copied\n    return bag\n}", want: "move-only"},
	})
}

// P18-35: the § 120 allocation-plan matrix. Append publishes an explicit
// plan with core::AllocationError and CollectionError; a reachable
// allocation under the noalloc profile is a compile-time error while the
// empty default stays valid; an Arena context gives Arena authority without
// a per-array reclamation plan; and no allocator is selected in Sema. Two
// rows are named boundaries: an individually reclaimable allocator context
// is never selected by the profile model (MD-034 configuration gap), and
// element alignment is a layout fact (P24) the plan does not carry.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 17–31, 56–57, 120
func TestPackage18AllocationPlanMatrix(t *testing.T) {
	runPackage18Cases(t, []package18Case{
		{name: "no allocation context is a compile-time error", profile: "noalloc", source: "fn Grow() Result[void, CollectionError] {\n    let mut values: int[]\n    try values.Append(1)\n    return Ok()\n}\n\nfn main() int {\n    discard Grow()\n    return 0\n}", want: "noalloc profile requires an allocation-free reachable graph"},
		{name: "empty default works in noalloc", profile: "noalloc", source: "fn main() int {\n    let mut values: int[]\n    discard values.Len\n    return 0\n}"},
	})
	parsed := parser.New(lexer.NewWithFile("module main\n\nfn Fill() Result[void, CollectionError] {\n    let mut values: int[]\n    try values.Append(1)\n    return Ok()\n}\n", "plan.sec")).Parse()
	analyzer := NewAnalyzer()
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(analyzer.allocationOperationPlans) != 1 {
		t.Fatalf("allocation-capable Append has %d plans, want one explicit plan", len(analyzer.allocationOperationPlans))
	}
	for _, plan := range analyzer.allocationOperationPlans {
		allocationError := plan.AllocationFailureType
		if allocationError.Name != "AllocationError" || !strings.Contains(strings.Join(allocationError.EnumValues, ","), "InvalidSize") {
			t.Errorf("AllocationError identity = %s %v, want core AllocationError with InvalidSize", allocationError.Name, allocationError.EnumValues)
		}
		if plan.FailureType.Name != "CollectionError" || !plan.Fallible || !plan.MayAllocate {
			t.Errorf("Append plan failure = %s fallible=%t may-allocate=%t", plan.FailureType.Name, plan.Fallible, plan.MayAllocate)
		}
		if plan.Context.Origin != StorageOriginArena || plan.ReclamationAuthority != ReclamationArena || plan.ReclamationPlan != 0 {
			t.Errorf("hosted Arena context = origin %s authority %s plan %d; want Arena, no per-array reclamation", plan.Context.Origin, plan.ReclamationAuthority, plan.ReclamationPlan)
		}
		if (plan.ReclamationPlan != 0) != (plan.ReclamationAuthority == ReclamationOwningValue) {
			t.Error("a reclamation plan must pair exactly with owning-value authority")
		}
	}
}

const package18Handle = "type Handle struct {\n    id: int,\n}\n\nimpl Handle {\n    free {\n    }\n}\n\n"

// P18-37/38/41 (Sema rows of §§ 122, 123, 126): a move-only element cannot
// be read by copy; a live slice or element reference blocks Append, which
// may relocate the backing; once the slice's last use is behind it growth is
// allowed; an element is replaced through a mutable slice of the owner.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 38, 43, 77, 122, 123, 126
func TestPackage18BorrowAndElementMatrix(t *testing.T) {
	body := func(code string) string { return package18Handle + "fn F() void {\n" + code + "\n}" }
	runPackage18Cases(t, []package18Case{
		{name: "move-only element read rejected", source: body("    let mut handles: Handle[]\n    let handle := handles[0]\n    discard handle"), want: "cannot copy move-only value handles[0]"},
		{name: "slice prevents relocating structural mutation", source: body("    let mut values: int[]\n    let view := ref values[..]\n    try values.Append(1) { Err(_) => { return } }\n    discard view.Len"), want: "cannot Append values while it is shared borrowed"},
		{name: "live direct reference blocks invalidating growth", source: body("    let mut values: int[]\n    let element := ref values[0]\n    try values.Append(1) { Err(_) => { return } }\n    discard *element"), want: "cannot Append values while it is shared borrowed"},
		{name: "slice ends then growth allowed", source: body("    let mut values: int[]\n    let view := ref values[..]\n    discard view.Len\n    try values.Append(1) { Err(_) => { return } }")},
		{name: "element replacement under legal borrow mode", source: body("    let mut values: int[]\n    let exclusive := ref mut values[..]\n    exclusive[0] = 4")},
	})
}

// P18-44 (§ 132): invented dynamic-array surface is rejected, never silently
// accepted: Push, Reserve, Capacity(), capacity, Clone(), element literal
// syntax that is not defined, and implicit fixed-array-to-owning and
// slice-to-owning conversions. Append is defined by collections.md § 6.4 and
// is accepted (TestPackage18AllocationPlanMatrix).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — § 132
func TestPackage18UnsupportedSurface(t *testing.T) {
	body := func(code string) string { return "fn F(view: ref int[], fixed: int[3]) void {\n" + code + "\n}" }
	runPackage18Cases(t, []package18Case{
		{name: "Push", source: body("    let mut values: int[]\n    values.Push(1)"), want: "values.Push"},
		{name: "Reserve", source: body("    let mut values: int[]\n    discard values.Reserve(4)"), want: "values.Reserve"},
		{name: "Capacity()", source: body("    let mut values: int[]\n    discard values.Capacity()"), want: "values.Capacity"},
		{name: "capacity", source: body("    let mut values: int[]\n    discard values.capacity"), want: "unknown member capacity"},
		{name: "Clone()", source: body("    let mut values: int[]\n    let copy := values.Clone()\n    discard copy"), want: "values.Clone"},
		{name: "undefined element literal", source: body("    let values: int[] := [1, 2]\n    discard values"), want: "owning dynamic-array literals with elements are not defined"},
		{name: "implicit fixed-array-to-owning conversion", source: body("    let values: int[] := fixed\n    discard values"), want: "cannot initialize int[] with int[3]"},
		{name: "implicit slice-to-owning conversion", source: body("    let values: int[] := view\n    discard values"), want: "cannot initialize int[] with ref int[]"},
	})
}

// P18-25 (§§ 106, 109): T[] is not an FFI-stable descriptor and has no
// equality; both are rejected in Sema rather than inferred.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 106, 109
func TestPackage18BoundarySurface(t *testing.T) {
	runPackage18Cases(t, []package18Case{
		{name: "FFI boundary", source: "extern \"C\" fn native(values: int[]) void", want: "cannot cross the C ABI boundary"},
		{name: "no equality", source: "fn F() bool {\n    let mut a: int[]\n    let mut b: int[]\n    return a == b\n}", want: "cannot compare int[] and int[]"},
	})
}
