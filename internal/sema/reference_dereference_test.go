package sema

import (
	"strings"
	"testing"
)

// MD-063: `*r` reads or replaces the whole referent of a safe reference;
// the holder `r` itself is never implicitly dereferenced.
func TestExplicitDereferenceAcceptsReadsWritesAndProjections(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
    Id: int,
}

fn Increment(value: ref mut int) void {
    *value += 1
}

fn Read(r: ref int) int {
    return *r * 2
}

fn Same(a: ref int, b: ref int) bool {
    return *a == *b
}

fn Project(s: ref Session) int {
    return (*s).Id + s.Id
}

fn Replace(s: ref mut Session, next: Session) void {
    *s <- next
}

fn Main() int {
    let mut number := 10
    let r := ref mut number
    *r = 15
    *r += 1
    let copy := *r
    Increment(number)
    return copy + Read(number)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestExplicitDereferenceRejectsHolderAndAuthorityMisuse(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
    Id: int,
}

fn Consume(value: int) void {
}

fn SharedWrite(r: ref int) void {
    *r = 25
}

fn HolderCompound(r: ref mut int) void {
    r += 1
}

fn HolderArithmetic(r: ref int) int {
    return r + 1
}

fn HolderValue(r: ref int) void {
    Consume(r)
}

fn HolderWrite(r: ref mut int) void {
    let mut q :<- r
    q = 25
}

fn OptionOperand(o: Option[ref int]) int {
    return *o
}

fn ValueOperand(v: int) int {
    return *v
}

fn MoveOut(r: ref mut int) void {
    let moved :<- *r
}

fn CopyMoveOnly(s: ref Session) void {
    let copy := *s
}

fn SliceView(values: ref int[]) void {
    let all := *values
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"cannot assign through shared reference r: ref int grants no write authority at 13:5",
		"compound assignment += targets reference holder r of type ref mut int, not its referent",
		"operator + requires numeric operands at 21:14",
		"argument 1 to Consume must be int, got ref int at 25:13",
		"cannot assign int to reference holder q of type ref mut int",
		"operator * requires a safe reference operand, got Option[ref int]",
		"operator * requires a safe reference operand, got int",
		"cannot move out of borrowed referent *r at 42:19",
		"cannot copy referent *s of move-only type Session through a reference at 46:17",
		"got ref int[]: a slice view has no whole referent value",
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
