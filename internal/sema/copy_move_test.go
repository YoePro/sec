package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
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

	arrayOfMutableRefs := Type{Name: "ref mut int[2]", Kind: ArrayType, Element: &mutableRef, ArrayLength: 2}
	if CopyClassificationOf(arrayOfMutableRefs) != CopyMoveOnly {
		t.Fatalf("array classification = %q, want %q", CopyClassificationOf(arrayOfMutableRefs), CopyMoveOnly)
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

impl Resource {
    fn Copy() int {
        return 1
    }

    fn Clone() Resource {
        return Resource { lock: Mutex(1) }
    }

    fn Duplicate() Result[Resource, string] {
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
		"Resource value first cannot be copied because field lock has non-copyable type Mutex[int]; use explicit move syntax :<- at 24:19",
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
		"use of moved value first at 11:19, previous declaration at 10:10",
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
