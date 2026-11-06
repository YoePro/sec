package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// MD-070: `RawPtr[T].AsRef` / `AsRefMut` are the unsafe spellings that
// form ordinary safe references from a raw pointer; `.Ptr` through a safe
// reference (`r.Ptr`, `(*r).Ptr`) addresses the referent's storage.
func TestRawPointerReferenceConversionsAreAccepted(t *testing.T) {
	input := rawPointerReferenceSource(t, "raw_pointer_reference_valid.sec")
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestRawPointerReferenceConversionsRejectMisuse(t *testing.T) {
	input := rawPointerReferenceSource(t, "raw_pointer_reference_invalid.sec")
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"RawPtr.AsRef requires unsafe",
		"RawPtr[void].AsRef cannot form a safe reference",
		"unknown member AsRef",
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
	if !strings.Contains(errors[4].Help, "pointer.AsRef") {
		t.Fatalf("implicit conversion help = %q", errors[4].Help)
	}
}

// MD-070 § 5(3): where Sema knows the raw pointer's origin (an immutable
// binding initialized from `place.Ptr`), forming `ref mut T` over a live
// borrow, `ref T` over a live mutable borrow, or any reference to an origin
// that no longer holds a value is rejected even inside unsafe.
func TestRawPointerReferenceConversionsCheckKnownOrigins(t *testing.T) {
	input := rawPointerReferenceSource(t, "raw_pointer_reference_origin_invalid.sec")
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

// Source fixtures exercise the canonical property syntax through ordinary Sema.
// Rules: rules/memory/raw_pointers.md §18A.
func rawPointerReferenceSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/tooling/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Old method forms cannot authorize or silently invoke an As* property.
// Rules: rules/corrections/applied/md070-as-property-naming-correction-20261106.md §2.
func TestRawPointerReferencePropertyCallsRejected(t *testing.T) {
	source := rawPointerReferenceSource(t, "raw_pointer_property_calls_invalid.sec")
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 3 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.RawPointerInvalidOperation || !strings.Contains(e.Message, "property and cannot be called") || !strings.Contains(e.Help, "without parentheses") {
			t.Fatal(e)
		}
	}
}
