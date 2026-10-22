package sema

import (
	"strings"
	"testing"
)

// MD-070: `RawPtr[T].AsRef()` / `AsRefMut()` are the unsafe spellings that
// form ordinary safe references from a raw pointer; `.Ptr` through a safe
// reference (`r.Ptr`, `(*r).Ptr`) addresses the referent's storage.
func TestRawPointerReferenceConversionsAreAccepted(t *testing.T) {
	input := `
module main

fn View(pointer: RawPtr[int]) ref int {
    return unsafe pointer.AsRef()
}

fn Bump(pointer: RawPtr[int]) void {
    let editor := unsafe pointer.AsRefMut()
    *editor += 1
}

fn Main(r: ref int) int {
    let mut value := 41
    unsafe {
        let pointer: RawPtr[int] := value.Ptr
        let fromReference: RawPtr[int] := r.Ptr
        let fromReferent: RawPtr[int] := (*r).Ptr
        discard fromReference
        discard fromReferent
        Bump(pointer)
    }
    return value
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestRawPointerReferenceConversionsRejectMisuse(t *testing.T) {
	input := `
module main

fn Outside(pointer: RawPtr[int]) ref int {
    return pointer.AsRef()
}

fn Opaque(pointer: RawPtr[void]) void {
    let r := unsafe pointer.AsRef()
}

fn NotRawPtr(r: ref int) void {
    let again := unsafe r.AsRef()
}

fn Null() int {
    unsafe {
        let pointer: RawPtr[int] := null
        let r := pointer.AsRef()
        return *r
    }
}

fn Implicit(pointer: RawPtr[int]) void {
    let r: ref int := pointer
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"RawPtr.AsRef requires unsafe",
		"RawPtr[void].AsRef cannot form a safe reference",
		"unknown function or type r.AsRef",
		"known to use a null address (RawPtr.AsRef)",
		"cannot initialize ref int with RawPtr[int]",
	}
	if len(errors) != len(want) {
		t.Fatalf("wrong sema error count. got=%d want=%d errors=%v", len(errors), len(want), errors)
	}
	for index, expected := range want {
		if !strings.Contains(errors[index].Error(), expected) {
			t.Fatalf("sema error %d = %q, want it to contain %q", index, errors[index].Error(), expected)
		}
	}
	if !strings.Contains(errors[4].Help, "unsafe pointer.AsRef()") {
		t.Fatalf("implicit conversion help = %q", errors[4].Help)
	}
}

// MD-070 § 5(3): where Sema knows the raw pointer's origin (an immutable
// binding initialized from `place.Ptr`), forming `ref mut T` over a live
// borrow, `ref T` over a live mutable borrow, or any reference to an origin
// that no longer holds a value is rejected even inside unsafe.
func TestRawPointerReferenceConversionsCheckKnownOrigins(t *testing.T) {
	input := `
module main

@noCopy
type Session struct {
    Id: int,
}

fn Conflicts() int {
    let mut x := 1
    let r := ref mut x
    unsafe {
        let p: RawPtr[int] := x.Ptr
        let alias := p.AsRefMut()
        *r = 2
        return *alias
    }
}

fn SharedWhileMutable() int {
    let mut x := 1
    let r := ref mut x
    unsafe {
        let p: RawPtr[int] := x.Ptr
        let view := p.AsRef()
        *r = 3
        return *view
    }
}

fn Dangling() int {
    let s := Session{Id: 1}
    unsafe {
        let p: RawPtr[Session] := s.Ptr
        let moved :<- s
        let view := p.AsRef()
        return view.Id + moved.Id
    }
}

fn Fine() int {
    let mut x := 1
    unsafe {
        let p: RawPtr[int] := x.Ptr
        let view := p.AsRef()
        return *view
    }
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"cannot form a mutable reference through RawPtr.AsRefMut to x while it is mutably borrowed",
		"cannot form a shared reference through RawPtr.AsRef to x while it is mutably borrowed",
		"RawPtr.AsRef through p would form a reference to s, which no longer holds a value",
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
