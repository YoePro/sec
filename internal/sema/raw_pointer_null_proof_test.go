package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A compiler-proven null dereference is a compile-time error even inside
// unsafe (raw_pointers.md § 7(8), § 34; unsafe.md § 8(6), § 21(2)). Proofs
// come from an immutable null-initialized binding and from dominating null
// tests on immutable bindings; unproven, mutable, and non-null paths stay
// accepted.
func TestProvenNullRawPointerDereferenceIsRejected(t *testing.T) {
	wrap := func(body string) string {
		return "module main\n\nfn Use(raw: RawPtr[int], other: RawPtr[int]) void {\n" + indentLines(body) + "\n}\n"
	}
	for _, body := range []string{
		"unsafe {\n    let empty: RawPtr[int] := null\n    discard empty.Read()\n}",
		"unsafe {\n    let empty: RawPtr[int] := null\n    empty.Write(1)\n}",
		"unsafe {\n    if raw is null {\n        discard raw.Read()\n    }\n}",
		"unsafe {\n    if raw is not null {\n        return\n    }\n    discard raw.VolatileRead()\n}",
		"unsafe {\n    if raw is not null {\n        discard raw.Read()\n    } else {\n        raw.VolatileWrite(2)\n    }\n}",
		"unsafe {\n    if !(raw is not null) && true {\n        discard raw.Read()\n    }\n}",
	} {
		errors := analyzeSource(t, wrap(body))
		if len(errors) != 1 || errors[0].ID != diagnostics.RawPointerNullDereference {
			t.Fatalf("%q errors = %v, want one %s", body, errors, diagnostics.RawPointerNullDereference)
		}
		if !strings.Contains(errors[0].Message, "is known to use a null address") {
			t.Fatalf("%q message = %q", body, errors[0].Message)
		}
	}
	for _, body := range []string{
		"unsafe {\n    discard raw.Read()\n}",
		"unsafe {\n    if raw is not null {\n        discard raw.Read()\n    }\n}",
		"unsafe {\n    if raw is null {\n        return\n    }\n    discard raw.Read()\n}",
		"unsafe {\n    let mut moving: RawPtr[int] := null\n    moving = raw\n    discard moving.Read()\n}",
		"unsafe {\n    if raw is null || true {\n        discard raw.Read()\n    }\n}",
		"unsafe {\n    if raw is null {\n        raw = other\n        discard raw.Read()\n    }\n}",
	} {
		if errors := analyzeSource(t, wrap(body)); len(errors) != 0 {
			t.Fatalf("%q errors = %v", body, errors)
		}
	}
}
