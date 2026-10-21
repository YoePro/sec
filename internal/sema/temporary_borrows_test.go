package sema

import (
	"strings"
	"testing"
)

const temporaryBorrowDeclarations = `
module main

type Counter struct {
    Count: int,
}

type Row struct {
    Id: int,
}

type Holder struct {
    Value: ref int,
}

impl Counter {
    fn GetCount() int {
        return self.Count
    }

    fn Increment() void {
        self.Count = self.Count + 1
    }

    fn CountRef() ref int {
        return ref self.Count
    }

    fn GetRow() Row {
        return Row{Id: self.Count}
    }
}

fn CreateCounter() Counter {
    return Counter{Count: 1}
}

fn Read(value: ref Counter) int {
    return value.Count
}

fn Bump(value: ref mut Counter) void {
    value.Count += 1
}

fn Field(value: ref Counter) ref int {
    return ref value.Count
}

fn Use(value: ref int) int {
    return *value
}
`

// MD-076 (borrowing.md § 5.4): fresh owned temporaries may be borrowed as
// call arguments and method receivers, including mutably and in chains, and a
// reference returned through such a borrow may be used inside its full
// expression.
func TestTemporaryCallBorrowsAreAccepted(t *testing.T) {
	input := temporaryBorrowDeclarations + `
fn Main() int {
    let a := Read(CreateCounter())
    Bump(CreateCounter())
    let b := CreateCounter().GetCount()
    CreateCounter().Increment()
    let row := CreateCounter().GetRow()
    let c := Use(Field(CreateCounter()))
    let d := *CreateCounter().CountRef()
    return a + b + c + d + row.Id
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestTemporaryCallBorrowsRejectEscapesAndExplicitBorrows(t *testing.T) {
	input := temporaryBorrowDeclarations + `
fn Escape() ref int {
    return Field(CreateCounter())
}

fn Main() void {
    let borrowed := ref CreateCounter()
    let writable := ref mut CreateCounter()
    let kept := Field(CreateCounter())
    let alsoKept := CreateCounter().CountRef()
    let shared := 1
    let mut holder := Holder{Value: ref shared}
    holder.Value = Field(CreateCounter())
    let fixed := CreateCounter()
    Bump(fixed)
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"function Escape cannot return reference to temporary CreateCounter()",
		"cannot borrow temporary expression; a reusable place is required",
		"cannot borrow temporary expression; a reusable place is required",
		"cannot keep reference returned by Field(CreateCounter()): it points into temporary CreateCounter(), which is cleaned at the end of this statement",
		"cannot keep reference returned by CreateCounter().CountRef(): it points into temporary CreateCounter()",
		"cannot keep reference returned by Field(CreateCounter())",
		"cannot create mutable reference to immutable variable fixed",
	}
	if len(errors) != len(want) {
		t.Fatalf("wrong sema error count. got=%d want=%d errors=%v", len(errors), len(want), errors)
	}
	for index, expected := range want {
		if !strings.Contains(errors[index].Error(), expected) {
			t.Fatalf("sema error %d = %q, want it to contain %q", index, errors[index].Error(), expected)
		}
	}
}

// A mutating method on a receiver that is neither writable nor a fresh
// temporary is reported, never rejected silently.
func TestMutatingMethodOnReadOnlyReceiverIsReported(t *testing.T) {
	input := temporaryBorrowDeclarations + `
fn Main(counter: ref Counter) void {
    counter.Increment()
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Error(), "Increment") {
		t.Fatalf("errors = %v, want one mutable-receiver error", errors)
	}
}
