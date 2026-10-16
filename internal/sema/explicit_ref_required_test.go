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
