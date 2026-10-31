package sema

import (
	"strings"
	"testing"
)

// MD-117 pack iteration in the template: the body is checked once with the
// opaque per-position element, valid for every specialization; loop modes
// follow MD-119 for reference elements; by-value copies of owned elements are
// checked per selected call.
// Rules: rules/declarations/functions.md — §38A(4)(6),(5)(3)–(7);
// rules/control-flow/flowcontrol_for.md — §13A; rules/memory/references.md — §3.4(10).
func TestTypePackIterationTemplate(t *testing.T) {
	valid := `module main
fn Touch[T](value: ref T) void {
}
fn TouchMut[T](value: ref mut T) void {
}
fn Shared[...Types](values: ...ref Types) uint {
    let mut n: uint := 0
    for index, value in values {
        Touch(value)
        if n > 3 { break }
        n = n + 1
    }
    for ref value in values {
        Touch(value)
    }
    return n
}
fn Exclusive[...Types](values: ...ref mut Types) void {
    for ref mut value in values {
        TouchMut(value)
    }
}
fn Owned[...Types](values: ...Types) void {
    for value in values {
    }
    for ref value in values {
        Touch(value)
    }
}
fn main() int {
    let mut a := 1
    let mut b := true
    let shared := Shared(ref a, ref b)
    Exclusive(ref mut a, ref mut b)
    Owned(1, true)
    return 0
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)
	for _, test := range []struct{ name, source, want string }{
		{"element arithmetic", "fn F[...Types](values: ...ref Types) void {\n    for ref value in values {\n        let x := value + 1\n    }\n}\n", "operator + requires numeric operands"},
		{"unknown member", "fn F[...Types](values: ...ref Types) void {\n    for ref value in values {\n        value.Foo()\n    }\n}\n", "unknown function or type value.Foo"},
		{"ref mut over shared pack", "fn F[...Types](values: ...ref Types) void {\n    for ref mut value in values {\n    }\n}\n", "needs a ...ref mut pack"},
		{"copy of ref mut holder", "fn F[...Types](values: ...ref mut Types) void {\n    for value in values {\n    }\n}\n", "cannot copy a ref mut pack element"},
		{"move-only by value", "@noCopy\ntype Handle struct { Id: int }\nfn F[...Types](values: ...Types) void {\n    for value in values {\n    }\n}\nfn main() int {\n    let h := Handle { Id: 1 }\n    F(1, <-h)\n    return 0\n}\n", "position 1 is Handle, which is not implicitly copyable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.source)
			found := false
			for _, err := range errors {
				found = found || strings.Contains(err.Message, test.want)
			}
			if !found {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}
