package sema

import (
	"strings"
	"testing"
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

func TestMoveOnlyLocalMoveMarksSourceUnavailable(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let first := ref mut value
    let second := first
    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 8:18, previous declaration at 7:19",
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

func TestMovedMutableLocalCanBeReinitialized(t *testing.T) {
	input := `
module main

fn Test() void {
    let mut value := 1
    let mut first := ref mut value
    Take(first)
    first = ref mut value
    let third := first
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
        let second := first
    }

    let third := first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"use of moved value first at 12:18, previous declaration at 9:23",
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
    let second := first
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
