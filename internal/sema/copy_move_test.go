package sema

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestCopyClassificationPrimitiveReferenceAndAggregates(t *testing.T) {
	intType := Type{Name: "int", Kind: IntType}
	sharedRef := Type{Name: "ref int", Kind: ReferenceType, Element: &intType}
	mutableRef := Type{Name: "ref mut int", Kind: ReferenceType, Element: &intType, ReferenceMutable: true}

	if CopyClassificationOf(intType) != CopyTrivial {
		t.Fatalf("int copy classification = %q, want %q", CopyClassificationOf(intType), CopyTrivial)
	}
	if CopyClassificationOf(sharedRef) != CopyTrivial {
		t.Fatalf("ref int copy classification = %q, want %q", CopyClassificationOf(sharedRef), CopyTrivial)
	}
	if CopyClassificationOf(mutableRef) != CopyMoveOnly {
		t.Fatalf("ref mut int copy classification = %q, want %q", CopyClassificationOf(mutableRef), CopyMoveOnly)
	}

	copyableStruct := Type{
		Name: "Copyable",
		Kind: StructType,
		Fields: []StructField{
			{Name: "value", Type: intType},
			{Name: "view", Type: sharedRef},
		},
	}
	if CopyClassificationOf(copyableStruct) != CopyTrivial {
		t.Fatalf("copyable struct classification = %q, want %q", CopyClassificationOf(copyableStruct), CopyTrivial)
	}

	moveOnlyStruct := Type{
		Name: "MoveOnly",
		Kind: StructType,
		Fields: []StructField{
			{Name: "value", Type: mutableRef},
		},
	}
	if CopyClassificationOf(moveOnlyStruct) != CopyMoveOnly {
		t.Fatalf("move-only struct classification = %q, want %q", CopyClassificationOf(moveOnlyStruct), CopyMoveOnly)
	}

	arrayOfMutableRefs := NewFixedArrayType(mutableRef, big.NewInt(2))
	if CopyClassificationOf(arrayOfMutableRefs) != CopyMoveOnly {
		t.Fatalf("array classification = %q, want %q", CopyClassificationOf(arrayOfMutableRefs), CopyMoveOnly)
	}
}

func TestOwningDynamicArrayAndArenaTraits(t *testing.T) {
	intType := Type{Name: "int", Kind: IntType}
	dynamic := NewDynamicArrayType(intType)
	fixed := NewFixedArrayType(intType, big.NewInt(4))
	arena := Type{Name: "Arena", Kind: StructType, Intrinsic: true}

	if got := CopyClassificationOf(dynamic); got != CopyMoveOnly {
		t.Fatalf("dynamic array classification = %q, want %q", got, CopyMoveOnly)
	}
	if TriviallyDestructible(dynamic) || EqualityComparable(dynamic) {
		t.Fatal("owning dynamic array must require destruction and have no ordinary equality")
	}
	if got := CopyClassificationOf(fixed); got != CopyTrivial || !TriviallyDestructible(fixed) || !EqualityComparable(fixed) {
		t.Fatalf("fixed array traits changed: copy=%q destructible=%v comparable=%v", got, TriviallyDestructible(fixed), EqualityComparable(fixed))
	}
	if got := CopyClassificationOf(arena); got != CopyMoveOnly || TriviallyDestructible(arena) {
		t.Fatalf("Arena traits = copy %q, trivial destruction %v", got, TriviallyDestructible(arena))
	}
	if canInitialize(dynamic, fixed, nil) || canInitialize(fixed, dynamic, nil) {
		t.Fatal("fixed and dynamic arrays must not initialize each other implicitly")
	}
	if !canInitialize(dynamic, dynamic, nil) {
		t.Fatal("matching dynamic array owners must remain initialization-compatible")
	}
}

func TestCompilerKnownNonCopyableClassification(t *testing.T) {
	mutex := Type{Name: "Mutex", Kind: StructType}
	if got := CopyClassificationOf(mutex); got != CopyNonCopyable {
		t.Fatalf("Mutex copy classification = %q, want %q", got, CopyNonCopyable)
	}
	if !requiresOwnershipTransfer(mutex) {
		t.Fatal("non-copyable Mutex must transfer ownership in consuming contexts")
	}
}

func TestCopyRestrictionCauseDistinguishesNominalPolicyAndCompilerKnownType(t *testing.T) {
	policyType := Type{
		Name:                  "SessionID",
		Kind:                  StructType,
		ExplicitlyNonCopyable: true,
		Fields: []StructField{
			{Name: "value", Type: Type{Name: "uint64", Kind: UintType}},
		},
	}
	if got := CopyClassificationOf(policyType); got != CopyNonCopyable {
		t.Fatalf("explicit nominal policy classification = %q, want %q", got, CopyNonCopyable)
	}
	if got := nonCopyableCause(policyType); got != "SessionID explicitly forbids implicit copy through @noCopy" {
		t.Fatalf("explicit nominal policy cause = %q", got)
	}

	mutex := Type{Name: "Mutex", Kind: StructType, TypeArgs: []Type{{Name: "int", Kind: IntType}}}
	if got := nonCopyableCause(mutex); got != "Mutex[int] is compiler-known non-copyable" {
		t.Fatalf("compiler-known cause = %q", got)
	}
}

func TestNoCopyAttributeClassifiesNominalTypeForms(t *testing.T) {
	input := `
module main

@noCopy
type SessionID struct {
    value: uint64,
}

@noCopy
enum State {
    Ready,
}

@noCopy
type Code int

type WrappedCode Code

@noCopy
type Flags register[8] {
    Enabled: bit,
    _: bit[7],
}

@noCopy
type Choice union {
    None,
    Some(int),
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	for _, name := range []string{"SessionID", "State", "Code", "WrappedCode", "Flags", "Choice"} {
		typ, ok := analyzer.Types()[name]
		if !ok {
			t.Fatalf("missing type %s", name)
		}
		if !typ.ExplicitlyNonCopyable || CopyClassificationOf(typ) != CopyNonCopyable {
			t.Fatalf("type %s did not retain @noCopy: %+v", name, typ)
		}
	}
	if got := analyzer.Types()["WrappedCode"].NoCopyPolicyOrigin; got != "Code" {
		t.Fatalf("WrappedCode @noCopy origin = %q, want Code", got)
	}
}

func TestNoCopyAttributeRejectsImplicitCopy(t *testing.T) {
	input := `
module main

@noCopy
type SessionID struct {
    value: uint64,
}

fn Test() void {
    let first := SessionID { value: 1 }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"SessionID value first cannot be copied because SessionID explicitly forbids implicit copy through @noCopy; use explicit move syntax :<- at 11:19",
	})
}

func TestNoCopyAttributeAllowsExplicitMoveAndInvalidatesSource(t *testing.T) {
	input := `
module main

@noCopy
type SessionID struct {
    value: uint64,
}

fn Test() void {
    let first := SessionID { value: 1 }
    let second :<- first
    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 12:18, previous declaration at 11:20",
	})
}

func TestNoCopyAttributeSurvivesGenericInstantiation(t *testing.T) {
	input := `
module main

@noCopy
type Box[T] struct {
    value: T,
}

fn Test() void {
    let first := Box[int] { value: 1 }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Box[int] value first cannot be copied because Box[int] explicitly forbids implicit copy through @noCopy; use explicit move syntax :<- at 11:19",
	})
}

func TestNoCopyFieldDiagnosticPreservesNominalPolicyCause(t *testing.T) {
	input := `
module main

@noCopy
type SessionID struct {
    value: uint64,
}

type Container struct {
    session: SessionID,
}

fn Test() void {
    let first := Container { session: SessionID { value: 1 } }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Container value first cannot be copied because field session has type SessionID, which explicitly forbids implicit copy through @noCopy; use explicit move syntax :<- at 15:19",
	})
}

func TestCompilerKnownNonCopyableLocalRequiresExplicitMove(t *testing.T) {
	input := `
module main

fn Test() void {
    let first := Mutex(1)
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Mutex[int] value first cannot be copied because Mutex[int] is compiler-known non-copyable; use explicit move syntax :<- at 6:19",
	})
	if errors[0].ID != diagnostics.ImplicitMoveDisallowed {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.ImplicitMoveDisallowed)
	}
}

func TestExplicitMoveOfCompilerKnownNonCopyableLocalMarksSourceUnavailable(t *testing.T) {
	input := `
module main

fn Test() void {
    let first := Mutex(1)
    let second :<- first
    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 7:18, previous declaration at 6:20",
	})
}

func TestMoveOnlyLocalMoveMarksSourceUnavailable(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref mut value
    let second :<- first
    let third :<- first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 8:19, previous declaration at 7:20",
	})
}

func TestImplicitCopyOfSharedReferenceIsAllowed(t *testing.T) {
	input := `
module main

fn Test() void {
    let value := 1
    let first := ref value
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestOrdinaryCopyOfMoveOnlyLocalRequiresExplicitMove(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref mut value
    let second := first
    Borrow(first)
}

fn Borrow(ref mut value: int) void {
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot copy move-only value first; use explicit move syntax :<- at 7:19",
	})
	if errors[0].ID != diagnostics.ImplicitMoveDisallowed {
		t.Fatalf("wrong diagnostic ID. got=%q want=%q", errors[0].ID, diagnostics.ImplicitMoveDisallowed)
	}
	if errors[0].Help != "use `let destination :<- source` to transfer ownership" {
		t.Fatalf("wrong diagnostic help. got=%q", errors[0].Help)
	}
}

func TestExplicitMoveOfCopyableLocalMarksSourceUnavailable(t *testing.T) {
	input := `
module main

fn Test() void {
    let first := 1
    let second :<- first
    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 7:18, previous declaration at 6:20",
	})
}

func TestGenericValueWithoutCopyProofRejectsImplicitCopy(t *testing.T) {
	input := `
module main

fn Duplicate[T](value: T) void {
    let copy := value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot copy value value because generic copyability has not been proven; use explicit move syntax :<- at 5:17",
	})
}

func TestGenericValueWithoutCopyProofAcceptsExplicitMove(t *testing.T) {
	input := `
module main

fn Transfer[T](value: T) T {
    let result :<- value
    return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDerivedCopyableStructCopiesSuccessfully(t *testing.T) {
	input := `
module main

type Point struct {
    x: int,
    y: int,
}

fn Test() void {
    let first := Point { x: 1, y: 2 }
    let second := first
    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestStructContainingNonCopyableFieldRejectsCopy(t *testing.T) {
	input := `
module main

type Locked struct {
    lock: Mutex[int],
}

fn Test() void {
    let first := Locked { lock: Mutex(1) }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Locked value first cannot be copied because field lock has non-copyable type Mutex[int]; use explicit move syntax :<- at 10:19",
	})
}

func TestNamedWrapperDoesNotBypassNonCopyableField(t *testing.T) {
	input := `
module main

type Locked struct {
    lock: Mutex[int],
}

type Wrapper struct {
    value: Locked,
}

fn Test() void {
    let first := Wrapper { value: Locked { lock: Mutex(1) } }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Wrapper value first cannot be copied because field value has non-copyable type Locked; use explicit move syntax :<- at 14:19",
	})
}

func TestNamedDuplicationMethodsDoNotEnableImplicitCopy(t *testing.T) {
	input := `
module main

type Resource struct {
    lock: Mutex[int],
}

enum DuplicateError error { Failed }

impl Resource {
    fn Copy() int {
        return 1
    }

    fn Clone() Resource {
        return Resource { lock: Mutex(1) }
    }

    fn Duplicate() Result[Resource, DuplicateError] {
        return Ok(Resource { lock: Mutex(1) })
    }
}

fn Test() void {
    let first := Resource { lock: Mutex(1) }
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"Resource value first cannot be copied because field lock has non-copyable type Mutex[int]; use explicit move syntax :<- at 26:19",
	})
}

func TestMoveAssignmentMarksSourceUnavailable(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut left := 1
    let mut right := 2
    right <- left
    let again := left
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value left at 8:18, previous declaration at 7:14",
	})
}

func TestMoveAssignmentRejectsSelfTransfer(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    value <- value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot move value value into itself at 6:14",
	})
}

func TestMovedMutableLocalCanBeReinitialized(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let mut first := ref mut value
    Take(first)
    first = ref mut value
    let third :<- first
}

fn Take(value: ref mut int) void {
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMoveInOneBranchMarksValuePossiblyMoved(t *testing.T) {
	input := `
module main

fn Test(condition: bool) void {
    let mut value := 1
    let first := ref mut value

    if condition {
        let second :<- first
    }

    let third :<- first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 12:19, previous declaration at 9:24",
	})
}

func TestByValueFunctionArgumentMovesMoveOnlyLocal(t *testing.T) {
	input := `
module main

fn Take(value: ref mut int) void {
}

fn Test() void {
    let mut value := 1
    let first := ref mut value
    Take(first)
    let second := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot copy move-only value first; use explicit move syntax :<- at 11:19",
	})
}

func TestRefParameterDoesNotMoveMoveOnlyLocal(t *testing.T) {
	input := `
module main

fn Borrow(ref mut value: int) void {
}

fn Test() void {
    let mut value := 1
    let first := ref mut value
    Borrow(first)
    let second :<- first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSharedBorrowsMayCoexist(t *testing.T) {
	input := `
module main

fn Test() void {
    let value := 1
    let first := ref value
    let second := ref value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMutableBorrowConflictsWithSharedBorrow(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref value
    let second := ref mut value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to value while it is already borrowed at 7:19, previous declaration at 6:18",
	})
}

func TestSharedBorrowConflictsWithMutableBorrow(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref mut value
    let second := ref value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create shared reference to value while it is mutably borrowed at 7:19, previous declaration at 6:18",
	})
}

func TestDirectReadConflictsWithMutableBorrow(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref mut value
    let copy := value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot read value while it is mutably borrowed at 7:17, previous declaration at 6:18",
	})
}

func TestAssignmentConflictsWithSharedBorrow(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref value
    value = 2
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to value while it is shared borrowed at 7:5, previous declaration at 6:18",
	})
}

func TestPlaceBorrowsPermitDisjointStructFields(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let left := ref mut pair.left
	let right := ref mut pair.right
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceBorrowPermitsAssignmentToDisjointStructField(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let left := ref pair.left
	pair.right = 3
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceBorrowConflictsWithOverlappingFieldAssignment(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let left := ref pair.left
	pair.left = 3
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to pair.left while it is shared borrowed at 12:7, previous declaration at 11:14",
	})
}

func TestPlaceBorrowsDistinguishConstantArrayIndices(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2]
	let first := ref mut values[0]
	let second := ref mut values[1]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReferenceAliasBorrowsConflictOnSameCanonicalField(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let alias := ref mut pair
	let first := ref mut alias.left
	let second := ref mut alias.left
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to pair.left while it is already borrowed at 13:16, previous declaration at 12:15",
	})
}

func TestReferenceAliasBorrowsPermitDisjointCanonicalFields(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let alias := ref mut pair
	let left := ref mut alias.left
	let right := ref mut alias.right
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSharedReferenceAliasDoesNotGrantMutableFieldAccess(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

fn Test() void {
	let mut pair := Pair { left: 1 }
	let alias := ref pair
	let invalid := ref mut alias.left
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to immutable place pair.left at 11:17",
	})
}

func TestProjectedReferenceAliasUsesOriginalPlace(t *testing.T) {
	input := `
module main

type Inner struct {
	left: int,
}

type Pair struct {
	inner: Inner,
}

fn Test() void {
	let mut pair := Pair { inner: Inner { left: 1 } }
	let alias := ref mut pair.inner
	let first := ref mut alias.left
	let second := ref mut alias.left
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to pair.inner.left while it is already borrowed at 16:16, previous declaration at 15:15",
	})
}

func TestConstantIndexReferenceAliasKeepsIndexProvenance(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2]
	let alias := ref mut values
	let first := ref mut alias[0]
	let second := ref mut alias[1]
	let duplicate := ref mut alias[0]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[0] while it is already borrowed at 9:19, previous declaration at 7:15",
	})
}

func TestMovedMutableReferenceKeepsCanonicalProvenance(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let alias := ref mut pair
	let moved :<- alias
	let first := ref mut moved.left
	let duplicate := ref mut moved.left
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to pair.left while it is already borrowed at 14:19, previous declaration at 13:15",
	})
}

func TestMovedMutableReferenceKeepsOwnerBorrowActive(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

fn Test() void {
	let mut pair := Pair { left: 1 }
	let alias := ref mut pair
	let moved :<- alias
	pair.left = 2
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to pair.left while it is mutably borrowed at 12:7, previous declaration at 10:15",
	})
}

func TestPlaceBorrowsConservativelyOverlapDynamicIndices(t *testing.T) {
	input := `
module main

fn Test(firstIndex: int, secondIndex: int) void {
	let mut values := [1, 2]
	let first := ref mut values[firstIndex]
	let second := ref mut values[secondIndex]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[*] while it is already borrowed at 7:16, previous declaration at 6:15",
	})
}

func TestWholePlaceBorrowOverlapsFieldBorrow(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
	right: int,
}

fn Test() void {
	let mut pair := Pair { left: 1, right: 2 }
	let left := ref pair.left
	let whole := ref mut pair
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to pair while it is already borrowed at 12:15, previous declaration at 11:14",
	})
}

func TestBorrowRequiresReusablePlace(t *testing.T) {
	input := `
module main

fn Create() int {
	return 1
}

fn Test() void {
	let borrowed := ref Create()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot borrow temporary expression; a reusable place is required at 9:18",
	})
}

func TestPlaceMoveKeepsDisjointStructFieldAvailable(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test() void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	let second :<- pair.second
	discard first
	discard second
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceMoveRejectsMovedFieldAndWholeAggregateUse(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test() void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	let again :<- pair.first
	let whole :<- pair
	discard first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value pair.first at 20:21, previous declaration at 19:20",
		"cannot use partially moved value pair; place pair.first is unavailable at 21:16, previous declaration at 19:20",
	})
}

func TestPlaceFieldReinitializationRestoresAggregateAvailability(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test() void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	pair.first = Session { value: 3 }
	let whole :<- pair
	discard first
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestNestedPlaceMoveKeepsSiblingAvailableAndReinitializesParent(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

type Envelope struct {
	pair: Pair,
}

fn Test() void {
	let mut envelope := Envelope {
		pair: Pair {
			first: Session { value: 1 },
			second: Session { value: 2 },
		},
	}
	let first :<- envelope.pair.first
	let second :<- envelope.pair.second
	envelope.pair.first = Session { value: 3 }
	envelope.pair.second = Session { value: 4 }
	let whole :<- envelope.pair
	discard first
	discard second
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestNestedPlaceMoveMakesParentsUnavailable(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

type Envelope struct {
	pair: Pair,
}

fn Test() void {
	let mut envelope := Envelope {
		pair: Pair {
			first: Session { value: 1 },
			second: Session { value: 2 },
		},
	}
	let first :<- envelope.pair.first
	let pair :<- envelope.pair
	let whole :<- envelope
	discard first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value envelope.pair; place envelope.pair.first is unavailable at 26:24, previous declaration at 25:29",
		"cannot use partially moved value envelope; place envelope.pair.first is unavailable at 27:16, previous declaration at 25:29",
	})
}

func TestPlaceMoveRejectsFieldExtractionThroughReference(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test() void {
	let mut pair := Pair { first: Session { value: 1 } }
	let alias := ref mut pair
	let first :<- alias.first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"partial move requires independently tracked local struct storage at 16:6",
	})
}

func TestPlaceBranchMergeTracksMovesFromDifferentBranches(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test(condition: bool) void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	if condition {
		let moved :<- pair.first
		discard moved
	} else {
		let moved :<- pair.second
		discard moved
	}
	let firstAgain :<- pair.first
	let secondAgain :<- pair.second
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value pair.first at 26:26, previous declaration at 20:21",
		"use of moved value pair.second at 27:27, previous declaration at 23:21",
	})
}

func TestPlaceBranchMergeRestoresFieldWhenEveryBranchReinitializes(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test(condition: bool) void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	if condition {
		pair.first = Session { value: 3 }
	} else {
		pair.first = Session { value: 4 }
	}
	let whole :<- pair
	discard first
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceBranchMergeKeepsFieldUnavailableWhenOnlyOneBranchReinitializes(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn Test(condition: bool) void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	if condition {
		pair.first = Session { value: 3 }
	}
	let whole :<- pair
	discard first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value pair; place pair.first is unavailable at 23:16, previous declaration at 19:20",
	})
}

func TestPlaceLiteralBranchExcludesUnreachableMoveState(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test() void {
	let mut pair := Pair { first: Session { value: 1 } }
	if false {
		let moved :<- pair.first
		discard moved
	}
	let first :<- pair.first
	discard first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceLoopMergeKeepsEntryAndBodyMoveState(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
	second: Session,
}

fn MoveInLoop(condition: bool) void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	while condition {
		let first :<- pair.first
		discard first
	}
	let whole :<- pair
}

fn ReinitializeInLoop(condition: bool) void {
	let mut pair := Pair {
		first: Session { value: 1 },
		second: Session { value: 2 },
	}
	let first :<- pair.first
	while condition {
		pair.first = Session { value: 3 }
	}
	let whole :<- pair
	discard first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"place pair.first may be unavailable on a later loop iteration at 20:22, previous declaration at 20:21",
		"cannot use partially moved value pair; place pair.first is unavailable at 23:16, previous declaration at 20:21",
		"cannot use partially moved value pair; place pair.first is unavailable at 35:16, previous declaration at 31:20",
	})
}

func TestPlaceLoopBackedgeAcceptsReinitializationBeforeReuse(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test(condition: bool) void {
	let mut pair := Pair { first: Session { value: 1 } }
	while condition {
		pair.first = Session { value: 2 }
		let first :<- pair.first
		discard first
	}
	pair.first = Session { value: 3 }
	let whole :<- pair
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceLoopBackedgeSeparatesBreakExit(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test() void {
	let mut pair := Pair { first: Session { value: 1 } }
	while true {
		let first :<- pair.first
		discard first
		break
	}
	let whole :<- pair
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value pair; place pair.first is unavailable at 20:16, previous declaration at 16:21",
	})
}

func TestPlaceLoopBackedgeExcludesConditionalBreakPath(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test(looping: bool, exit: bool) void {
	let mut pair := Pair { first: Session { value: 1 } }
	while looping {
		if exit {
			let first :<- pair.first
			discard first
			break
		}
		pair.first = Session { value: 2 }
	}
	let whole :<- pair
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value pair; place pair.first is unavailable at 23:16, previous declaration at 17:22",
	})
}

func TestPlaceLoopBackedgeTracksContinueExit(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test(condition: bool) void {
	let mut pair := Pair { first: Session { value: 1 } }
	while condition {
		let first :<- pair.first
		discard first
		continue
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"place pair.first may be unavailable on a later loop iteration at 16:22, previous declaration at 16:21",
	})
}

func TestPlaceLoopBackedgeRechecksWhileCondition(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test() void {
	let mut pair := Pair { first: Session { value: 1 } }
	while pair.first.value > 0 {
		let first :<- pair.first
		discard first
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"place pair.first may be unavailable on a later loop iteration at 15:13, previous declaration at 16:21",
	})
}

func TestPlaceForLoopBackedgeRechecksMovedPlace(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Pair struct {
	first: Session,
}

fn Test() void {
	let mut pair := Pair { first: Session { value: 1 } }
	for index in 0..<2 {
		let first :<- pair.first
		discard first
		discard index
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"place pair.first may be unavailable on a later loop iteration at 16:22, previous declaration at 16:21",
	})
}

func TestUnionPayloadPlaceVariantsAreDisjoint(t *testing.T) {
	payloadType := Type{Name: "Session", Kind: StructType}
	root := Place{Root: "choice", Type: Type{Name: "Choice", Kind: UnionType}}
	some := unionPayloadPlace(root, "Some", payloadType, lexer.Token{})
	other := unionPayloadPlace(root, "Other", payloadType, lexer.Token{})

	if PlacesOverlap(some, other) {
		t.Fatal("different union variant payload places must be disjoint")
	}
	if !PlacesOverlap(root, some) {
		t.Fatal("whole union place must overlap its variant payload")
	}
	if some.String() != "choice.<Some>" {
		t.Fatalf("wrong union payload place: %q", some.String())
	}
}

func TestMatchMovesNonCopyableUnionPayloadFromReusableSubject(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Session { value: 1 })
	match choice {
		Some(session) => {
			discard session
		}
		None => {}
	}
	let whole :<- choice
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value choice; place choice.<Some> is unavailable at 22:16, previous declaration at 17:8",
	})
}

func TestUnionConstructorMovesNonCopyablePayloadSource(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn Test() void {
	let session := Session { value: 1 }
	let choice := Choice.Some(session)
	discard session
	discard choice
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value session at 17:10, previous declaration at 16:28",
	})
}

func TestMatchExpressionMergesUnionPayloadMoveState(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Session { value: 1 })
	let code := match choice {
		Some(session) => 1
		None => 0
	}
	let whole :<- choice
	discard code
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value choice; place choice.<Some> is unavailable at 20:16, previous declaration at 17:8",
	})
}

func TestMatchCopiesCopyableUnionPayload(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(1)
	match choice {
		Some(value) => {
			discard value
		}
		None => {}
	}
	let whole :<- choice
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchUnionPayloadReinitializationRestoresSubject(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Session { value: 1 })
	match choice {
		Some(session) => {
			choice = Choice.None
			discard session
		}
		None => {
			choice = Choice.None
		}
	}
	let whole :<- choice
	discard whole
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchUnionPayloadMoveConflictsWithSubjectBorrow(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Session { value: 1 })
	let view := ref choice
	match choice {
		Some(session) => {
			discard session
		}
		None => {}
	}
	discard view
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot move choice.<Some> while it is borrowed at 18:8, previous declaration at 16:14",
	})
}

func TestMatchCanForwardUnionPayloadFromTemporary(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

fn NewSession() Session {
	return Session { value: 1 }
}

fn Test() void {
	match Choice.Some(NewSession()) {
		Some(session) => {
			discard session
		}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestNestedUnionPayloadMovePreservesDisjointStructField(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
	value: int,
}

type Choice union {
	Some(Session),
	None,
}

type Wrapper struct {
	choice: Choice,
	count: int,
}

fn Test() void {
	let mut wrapper := Wrapper {
		choice: Choice.Some(Session { value: 1 }),
		count: 2,
	}
	match wrapper.choice {
		Some(session) => {
			discard session
		}
		None => {}
	}
	let count := wrapper.count
	let whole :<- wrapper
	discard count
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use partially moved value wrapper; place wrapper.choice.<Some> is unavailable at 31:16, previous declaration at 25:8",
	})
}

func TestLargeByValueArrayParameterWarns(t *testing.T) {
	input := `
module main

fn Process(values: int[16]) int {
    return values[0]
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	expected := `parameter "values" passes large array int[16] by value; consider ref int[16] or ref int[] at 4:12`
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	if warnings[0].Error() != expected {
		t.Fatalf("wrong warning. got=%q want=%q", warnings[0].Error(), expected)
	}
	if warnings[0].ID != "A2001" {
		t.Fatalf("wrong warning ID. got=%q want=A2001", warnings[0].ID)
	}
	if !strings.Contains(warnings[0].Help, "Pass the parameter by shared reference") {
		t.Fatalf("missing warning help. got=%q", warnings[0].Help)
	}
}

func TestLargeByValueStructParameterWarns(t *testing.T) {
	input := `
module main

type Frame struct {
    a: int,
    b: int,
    c: int,
    d: int,
    e: int,
    f: int,
    g: int,
    h: int,
}

fn Process(frame: Frame) int {
    return frame.a
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	expected := `parameter "frame" passes large value Frame by value; consider ref Frame at 15:12`
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	if warnings[0].Error() != expected {
		t.Fatalf("wrong warning. got=%q want=%q", warnings[0].Error(), expected)
	}
	if warnings[0].ID != "A2001" {
		t.Fatalf("wrong warning ID. got=%q want=A2001", warnings[0].ID)
	}
	if !strings.Contains(warnings[0].Help, "Pass the parameter by shared reference") {
		t.Fatalf("missing warning help. got=%q", warnings[0].Help)
	}
}

func TestStaticSlicePlaceOverlap(t *testing.T) {
	left := Place{Root: "values", Projections: []PlaceProjection{{
		Kind: PlaceSlice, SliceStart: 0, SliceEnd: 2, SliceStartKnown: true, SliceEndKnown: true,
	}}}
	right := Place{Root: "values", Projections: []PlaceProjection{{
		Kind: PlaceSlice, SliceStart: 2, SliceEnd: 4, SliceStartKnown: true, SliceEndKnown: true,
	}}}
	overlap := Place{Root: "values", Projections: []PlaceProjection{{
		Kind: PlaceSlice, SliceStart: 1, SliceEnd: 3, SliceStartKnown: true, SliceEndKnown: true,
	}}}
	inside := Place{Root: "values", Projections: []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: 1}}}
	outside := Place{Root: "values", Projections: []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: 4}}}
	empty := Place{Root: "values", Projections: []PlaceProjection{{
		Kind: PlaceSlice, SliceStart: 2, SliceEnd: 2, SliceStartKnown: true, SliceEndKnown: true,
	}}}

	if PlacesOverlap(left, right) {
		t.Fatal("adjacent static slices must be disjoint")
	}
	if !PlacesOverlap(left, overlap) || !PlacesOverlap(left, inside) {
		t.Fatal("intersecting slice ranges and contained indices must overlap")
	}
	if PlacesOverlap(left, outside) || PlacesOverlap(left, empty) {
		t.Fatal("outside indices and empty static slices must be disjoint")
	}
	if got := left.String(); got != "values[..<2]" {
		t.Fatalf("slice place string = %q, want values[..<2]", got)
	}
}

func TestPlaceBorrowsPermitDisjointStaticSlices(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let left := ref mut values[0..<2]
	let right := ref mut values[2..<4]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPlaceBorrowsRejectOverlappingStaticSlices(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let left := ref mut values[0..<3]
	let right := ref mut values[2..<4]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[2..<4] while it is already borrowed at 7:15, previous declaration at 6:14",
	})
}

func TestStaticSliceAndIndexBorrowOverlap(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let middle := ref mut values[1..<3]
	let outside := ref mut values[0]
	let inside := ref mut values[2]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[2] while it is already borrowed at 8:16, previous declaration at 6:16",
	})
}

func TestSliceAliasIndexProvenanceComposesRanges(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let view := ref mut values[1..<3]
	let first := ref mut view[0]
	let second := ref mut view[1]
	let outside := ref mut values[0]
	let duplicate := ref mut view[0]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[1] while it is already borrowed at 10:19, previous declaration at 7:15",
	})
}

func TestSliceAliasSubrangesComposeAndRemainDisjoint(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let view := ref mut values[1..<4]
	let left := ref mut view[0..<1]
	let right := ref mut view[1..<3]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInclusiveAndOpenStaticSlicesNormalizeToDisjointRanges(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2, 3, 4]
	let left := ref mut values[..1]
	let right := ref mut values[2..]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDynamicSliceRangesRemainConservativelyOverlapping(t *testing.T) {
	input := `
module main

fn Test(end: int) void {
	let mut values := [1, 2, 3, 4]
	let dynamic := ref mut values[0..<end]
	let fixed := ref mut values[2..<4]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to values[2..<4] while it is already borrowed at 7:15, previous declaration at 6:17",
	})
}

func TestEmptyStaticSliceDoesNotBorrowElements(t *testing.T) {
	input := `
module main

fn Test() void {
	let mut values := [1, 2]
	let empty := ref mut values[0..<0]
	let whole := ref mut values
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSharedUnionPayloadPatternBorrowBlocksVariantMutation(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

type Choice union {
	Some(Pair),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Pair { left: 1 })
	match choice {
		Some(ref payload) => {
			let value := payload.left
			choice = Choice.None
			discard value
		}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to choice while it is shared borrowed at 18:4, previous declaration at 16:12",
	})
}

func TestMutableUnionPayloadPatternBorrowAllowsMutationThroughBinding(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

type Choice union {
	Some(Pair),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(Pair { left: 1 })
	match choice {
		Some(ref mut payload) => {
			payload.left = 2
		}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMutableUnionPayloadPatternRequiresMutableSubject(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

type Choice union {
	Some(Pair),
	None,
}

fn Test() void {
	let choice: Choice := Choice.Some(Pair { left: 1 })
	match choice {
		Some(ref mut payload) => {}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to immutable place choice.<Some> at 16:16",
	})
}

func TestUnionPayloadPatternBorrowEndsAtArmExit(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(1)
	match choice {
		Some(ref value) => {
			let copy := value
			discard copy
		}
		None => {}
	}
	choice = Choice.None
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestUnionPayloadPatternBorrowRejectsTemporarySubject(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test() void {
	match Choice.Some(1) {
		Some(ref value) => {
			discard value
		}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot borrow union payload from temporary match subject at 11:12",
	})
}

func TestUnionPayloadPatternReferenceCannotEscapeBranch(t *testing.T) {
	input := `
module main

type Pair struct {
	left: int,
}

type Choice union {
	Left(Pair),
	Right(Pair),
}

fn Get(choice: Choice) ref Pair {
	return match choice {
		Left(ref value) => value
		Right(ref value) => value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"match expression cannot produce a branch-scoped union payload reference at 14:9, previous declaration at 15:12",
	})
}

func TestMutableUnionPayloadPatternConflictsWithExistingOwnerBorrow(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test() void {
	let mut choice: Choice := Choice.Some(1)
	let view := ref choice
	match choice {
		Some(ref mut value) => {}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot create mutable reference to choice.<Some> while it is already borrowed at 13:16, previous declaration at 11:14",
	})
}

func TestNestedUnionPayloadPatternBorrowUsesNestedPlace(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

type Wrapper struct {
	choice: Choice,
	count: int,
}

fn Test() void {
	let mut wrapper := Wrapper { choice: Choice.Some(1), count: 2 }
	match wrapper.choice {
		Some(ref value) => {
			wrapper.count = 3
			wrapper.choice = Choice.None
			discard value
		}
		None => {}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to wrapper.choice while it is shared borrowed at 19:12, previous declaration at 17:12",
	})
}

func TestUnionPayloadReferencePatternRequiresIdentifier(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test(choice: Choice) void {
	match choice {
		Some(ref 1) => {}
		None => {}
	}
}
`

	p := parser.New(lexer.New(input))
	result := p.Parse()
	if !result.HasErrors || len(p.Errors()) == 0 || !strings.Contains(p.Errors()[0], "match payload reference must bind an identifier") {
		t.Fatalf("parser errors = %v", p.Errors())
	}
}

func TestResultPayloadReferencePatternsBorrowActiveVariants(t *testing.T) {
	input := `
module main

enum TestError error { Failed }

fn Test(result: Result[int, TestError]) void {
	match result {
		Ok(ref value) => {
			let alias := value
			discard alias
		}
		Err(ref error) => {
			let alias := error
			discard alias
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestUnionPayloadReferenceCannotBeAssignedOutsideArm(t *testing.T) {
	input := `
module main

type Choice union {
	Some(int),
	None,
}

fn Test(choice: Choice, ref fallback: int) void {
	let mut holder := fallback
	match choice {
		Some(ref value) => {
			holder = value
		}
		None => {}
	}
	discard holder
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot store branch-scoped union payload reference outside its match arm at 13:13, previous declaration at 12:12",
	})
}

func TestAggregateContainingUnionPayloadReferenceCannotReturn(t *testing.T) {
	input := `
module main

type View struct {
	value: ref int,
}

type Choice union {
	Some(int),
	None,
}

fn Get(choice: Choice, ref fallback: int) View {
	match choice {
		Some(ref value) => {
			return View { value: value }
		}
		None => {
			return View { value: fallback }
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Get cannot return a value containing a branch-scoped union payload reference at 16:11, previous declaration at 15:12",
	})
}

func TestLoopBorrowStateIncludesContinueBackedgeAndPostLoopExit(t *testing.T) {
	input := `
module main

fn Test(condition: bool) void {
	let mut target := 1
	let other := 0
	let mut view: ref int := ref other
	while condition {
		view = ref target
		continue
	}
	target = 2
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to target while it is shared borrowed at 12:2, previous declaration at 9:10",
	})
}

func TestLoopBorrowFixedPointRejectsPreviousIterationConflict(t *testing.T) {
	input := `
module main

fn Test(condition: bool) void {
	let mut first := 1
	let mut second := 2
	let mut view: ref mut int := ref mut first
	while condition {
		view = ref mut second
	}
	second = 3
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to second while it is mutably borrowed at 11:2, previous declaration at 9:10",
	})
}

func TestLoopReferenceProvenanceJoinAllowsFiniteMultiOriginReborrow(t *testing.T) {
	input := `
module main

type Pair struct {
	value: int,
}

fn Test(condition: bool) void {
	let left := Pair { value: 1 }
	let right := Pair { value: 2 }
	let mut view: ref Pair := ref left
	while condition {
		if condition {
			view = ref right
		}
		let field := ref view.value
		discard field
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLoopReferenceProvenanceJoinKeepsIdenticalOrigin(t *testing.T) {
	input := `
module main

type Pair struct {
	value: int,
}

fn Test(condition: bool) void {
	let pair := Pair { value: 1 }
	let mut view: ref Pair := ref pair
	while condition {
		view = ref pair
		let field := ref view.value
		discard field
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLoopEdgeDoesNotRetainIterationLocalBorrowHolder(t *testing.T) {
	input := `
module main

fn Test(condition: bool) void {
	let mut target := 1
	while condition {
		let view := ref target
		discard view
		continue
	}
	target = 2
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBranchMultiOriginProvenanceSurvivesSharedReferenceCopy(t *testing.T) {
	input := `
module main

type Pair struct {
	value: int,
}

fn Test(condition: bool) void {
	let left := Pair { value: 1 }
	let right := Pair { value: 2 }
	let mut view: ref Pair := ref left
	if condition {
		view = ref right
	}
	let alias := view
	let field := ref alias.value
	discard field
	discard alias
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReferenceOriginJoinRetainsProjectedAlternativePlaces(t *testing.T) {
	intType := Type{Name: "int", Kind: IntType}
	pairType := Type{Name: "Pair", Kind: StructType, Fields: []StructField{{Name: "value", Type: intType}}}
	left := Place{Root: "left", Type: pairType, Mutable: true, Addressable: true}
	right := Place{Root: "right", Type: pairType, Mutable: true, Addressable: true}
	joined := mergeReferenceOriginStates(
		map[string]localReferenceOrigin{"view": localOriginWithPlaces(localReferenceOrigin{Name: "left", Local: true}, []Place{left})},
		map[string]localReferenceOrigin{"view": localOriginWithPlaces(localReferenceOrigin{Name: "right", Local: true}, []Place{right})},
	)
	origin := joined["view"]
	if origin.Unknown || !origin.Ambiguous || len(origin.Places) != 2 {
		t.Fatalf("expected finite two-place provenance, got %+v", origin)
	}

	analyzer := NewAnalyzer()
	analyzer.symbols = map[string]Symbol{
		"view": {Name: "view", Type: Type{Name: "ref Pair", Kind: ReferenceType, Element: &pairType}, Local: true},
	}
	analyzer.localRefContainers = joined
	analyzer.borrows = map[string][]borrowRecord{}
	expr := &ast.MemberExpression{
		Object:   &ast.Identifier{Value: "view"},
		Property: &ast.Identifier{Value: "value"},
	}
	place, ok := analyzer.resolvePlace(expr)
	if !ok {
		t.Fatal("failed to resolve projected multi-origin reference place")
	}
	alternatives := placeOriginAlternatives(place)
	if len(alternatives) != 2 || alternatives[0].String() != "left.value" || alternatives[1].String() != "right.value" {
		t.Fatalf("wrong projected alternatives: %#v", alternatives)
	}
	analyzer.registerBorrow("field", &ast.RefExpression{Value: expr})
	if len(analyzer.borrows["left"]) != 1 || len(analyzer.borrows["right"]) != 1 ||
		analyzer.borrows["left"][0].Place.String() != "left.value" || analyzer.borrows["right"][0].Place.String() != "right.value" {
		t.Fatalf("reborrow did not register every alternative: %#v", analyzer.borrows)
	}
	analyzer.errorKeys = map[string]bool{}
	analyzer.borrows["right"] = append(analyzer.borrows["right"], borrowRecord{
		Root: "right", Place: alternatives[1], Holder: "blocker", Kind: mutableBorrow,
	})
	if !analyzer.checkBorrowCreationPlace(place, false, lexer.Token{}) {
		t.Fatal("borrow checking ignored a conflicting alternative origin")
	}
}

func TestReferenceOriginJoinCapsPathExplosionAsUnknown(t *testing.T) {
	states := make([]map[string]localReferenceOrigin, 0, maxReferenceOriginAlternatives+1)
	for index := 0; index <= maxReferenceOriginAlternatives; index++ {
		place := Place{Root: fmt.Sprintf("owner%d", index), Addressable: true}
		states = append(states, map[string]localReferenceOrigin{
			"view": localOriginWithPlaces(localReferenceOrigin{}, []Place{place}),
		})
	}
	origin := mergeReferenceOriginStates(states...)["view"]
	if !origin.Unknown || !origin.Ambiguous || len(origin.Places) != 0 || origin.HasPlace {
		t.Fatalf("expected capped unknown provenance, got %+v", origin)
	}
	analyzer := NewAnalyzer()
	analyzer.localRefContainers = map[string]localReferenceOrigin{"view": origin}
	if !analyzer.referencePlaceOriginIsAmbiguous(&ast.Identifier{Value: "view"}) {
		t.Fatal("over-limit provenance must remain conservatively non-reborrowable")
	}
}
