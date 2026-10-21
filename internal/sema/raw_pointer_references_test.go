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
