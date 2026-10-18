package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// An owned value where a direct reference is required is rejected outside
// call arguments (S1169): a typed let, a module initializer, an assignment
// through a ref mut parameter, a return and a struct field initializer. The
// explicit borrow is then checked like any other, so a local escape is still
// caught (S1128/S1129), while call arguments keep their marker-free borrow
// and a static initializer may reference static storage.
//
// Rules:
//   - rules/memory/borrowing.md — §§ 5.1, 15.2
//   - rules/memory/references.md — § 5
func TestImplicitBorrowOutsideCallsIsRejected(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"typed let": {`module main
fn Use() int {
    let x := 1
    let r: ref int := x
    return 0
}
`, diagnostics.BorrowExplicitRefRequired},
		"module initializer": {`module main
let shared: int := 7
let mut slot: ref int := shared
`, diagnostics.BorrowExplicitRefRequired},
		"assignment through ref mut": {`module main
type View struct { value: ref int }
fn Store(out: ref mut View) void {
    let x := 10
    out.value = x
}
`, diagnostics.BorrowExplicitRefRequired},
		"return": {`module main
fn Get(p: ref int) ref int {
    let x := 1
    return x
}
`, diagnostics.BorrowExplicitRefRequired},
		"struct field": {`module main
type View struct { value: ref int }
fn Make() int {
    let x := 1
    let v := View { value: x }
    return 0
}
`, diagnostics.BorrowExplicitRefRequired},
		"explicit local store escapes": {`module main
type View struct { value: ref int }
fn Store(out: ref mut View) void {
    let x := 10
    out.value = ref x
}
`, diagnostics.EscapeOuterPlace},
		"explicit local return escapes": {`module main
fn Get(p: ref int) ref int {
    let x := 1
    return ref x
}
`, diagnostics.EscapeLocalStorage},
		"call argument borrows implicitly": {`module main
fn Read(p: ref int) int { return 0 }
fn Use() int {
    let x := 1
    return Read(x)
}
`, ""},
		"static reference to static": {`module main
let shared: int := 7
let mut slot: ref int := ref shared
`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, test.source)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("unexpected errors: %v", errors)
				}
				return
			}
			if len(errors) != 1 || errors[0].ID != test.want || errors[0].Help == "" {
				t.Fatalf("want one %s with help, got %v", test.want, errors)
			}
		})
	}
}

// MD-080: S1169 carries the canonical help text, the occurrence names the
// required reference type and distinguishes `ref` from `ref mut`, and the
// explicit spelling is accepted.
//
// Rules:
//   - rules/tooling/diagnostics.md — S1169 borrowing.explicit-ref-required
func TestExplicitRefRequiredCanonicalHelp(t *testing.T) {
	errors := analyzeSourceRaw(t, "module main\nfn Use() int {\n    let mut count: int := 10\n    let view: ref mut int := count\n    return 0\n}\n")
	if len(errors) != 1 || errors[0].ID != diagnostics.BorrowExplicitRefRequired {
		t.Fatalf("want one S1169, got %v", errors)
	}
	if errors[0].Help != "Only call arguments may borrow implicitly.\nWrite `ref value` or `ref mut value` to create an explicit reference." {
		t.Fatalf("non-canonical help %q", errors[0].Help)
	}
	if want := "ref mut int is required here, got owned int; write `ref mut count`"; errors[0].Message != want {
		t.Fatalf("message %q, want %q", errors[0].Message, want)
	}
	if errors := analyzeSourceRaw(t, "module main\nfn Use() int {\n    let count: int := 10\n    let countView: ref int := ref count\n    return 0\n}\n"); len(errors) != 0 {
		t.Fatalf("explicit reference rejected: %v", errors)
	}
}
