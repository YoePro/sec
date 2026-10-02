package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func analyzeNullFixture(t *testing.T, body string) []Error {
	t.Helper()
	return analyzeSourceRaw(t, `module main
type Device struct { id: int32 }
extern "C" fn Use(device: RawPtr[Device]) void
fn Get() RawPtr[Device] {
    unsafe {
        return null
    }
}
fn F(raw: RawPtr[Device], count: int) void {
    `+body+`
}
`)
}

// null is an unsafe-only raw-pointer sentinel with no standalone type: a
// RawPtr[T] target context (annotation, assignment, return, or parameter)
// types it, and raw pointers are tested with is null inside unsafe.
//
// Rules:
//   - rules/platform/ffi.md — §11 "null"
//   - rules/memory/raw_pointers.md — § 7(3), § 7(9)
func TestNullSentinelInRawPointerContexts(t *testing.T) {
	for _, body := range []string{
		"unsafe {\n        if raw is null {\n            return\n        }\n    }",
		"unsafe {\n        while raw is null {\n            return\n        }\n    }",
		"unsafe {\n        let value: RawPtr[Device] := null\n        discard value\n    }",
		"unsafe {\n        let mut value: RawPtr[Device] := raw\n        value = null\n        discard value\n    }",
		"unsafe {\n        Use(null)\n    }",
		"let null := 5\n    discard null",
	} {
		if errors := analyzeNullFixture(t, body); len(errors) != 0 {
			t.Fatalf("%q errors = %v, want none", body, errors)
		}
	}
}

func TestNullSentinelMisuseUsesOwningDiagnostics(t *testing.T) {
	tests := []struct {
		body   string
		wantID string
		want   string
	}{
		{"if raw is null {\n        return\n    }", diagnostics.NullOutsideUnsafe, "may be written only inside an unsafe context"},
		{"let value: RawPtr[Device] := null\n    discard value", diagnostics.NullOutsideUnsafe, "may be used only inside an unsafe context"},
		{"unsafe {\n        let value := null\n    }", diagnostics.NullWithoutRawPointerContext, "null has no standalone type"},
		{"unsafe {\n        let n: int := null\n        discard n\n    }", diagnostics.NullWithoutRawPointerContext, "needs a RawPtr[T] target context"},
		{"unsafe {\n        if raw == null {\n            return\n        }\n    }", diagnostics.NullEquality, "use is null"},
		{"if null != raw {\n        return\n    }", diagnostics.NullEquality, "use is null"},
		{"unsafe {\n        if count is null {\n            return\n        }\n    }", diagnostics.NullTestRequiresRawPointer, "got int"},
	}
	for _, test := range tests {
		errors := analyzeNullFixture(t, test.body)
		if len(errors) != 1 || errors[0].ID != test.wantID || errors[0].Help == "" || !strings.Contains(errors[0].Message, test.want) {
			t.Fatalf("%q errors = %+v, want one %s with help containing %q", test.body, errors, test.wantID, test.want)
		}
	}
}
