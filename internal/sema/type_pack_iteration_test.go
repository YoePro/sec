package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
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
		{"nested elements do not unify", "fn Same[T](x: ref T, y: ref T) void {\n}\nfn F[...Types](values: ...ref Types) void {\n    for ref a in values {\n        for ref b in values {\n            Same(a, b)\n        }\n    }\n}\n", "Same"},
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

// Per-call by-value loop diagnostics follow source order, independent of the
// analyzer's internal call-fact map.
func TestTypePackByValueDiagnosticsAreOrdered(t *testing.T) {
	source := `module main
@noCopy
type Handle struct { Id: int }
fn F[...Types](values: ...Types) void {
    for value in values {
    }
}
fn main() int {
    let a := Handle { Id: 1 }
    let b := Handle { Id: 2 }
    let c := Handle { Id: 3 }
    F(<-a)
    F(<-b)
    F(<-c)
    return 0
}
`
	for attempt := 0; attempt < 20; attempt++ {
		errors := analyzeSourceRaw(t, source)
		if len(errors) != 3 || errors[0].Line != 12 || errors[1].Line != 13 || errors[2].Line != 14 {
			t.Fatalf("attempt %d: %v", attempt, errors)
		}
	}
}

// MD-117 constant pack indexing: the index is compile-time-known, equal
// positions unify and different ones do not, and whether a position exists or
// may be copied is decided per selected call.
// Rules: rules/declarations/functions.md — §38A(5)(2),(5),(7)(4),(3)(5).
func TestTypePackIndexing(t *testing.T) {
	prelude := "module main\nfn Touch[T](value: ref T) void {\n}\nfn TouchMut[T](value: ref mut T) void {\n}\nfn Same[T](x: ref T, y: ref T) void {\n}\n@noCopy\ntype Handle struct { Id: int }\n"
	assertSemaErrors(t, analyzeSourceRaw(t, prelude+`fn S[...Types](values: ...ref Types) void {
    Touch(values[1])
    Same(values[0], values[0])
    let k := 1
    Touch(values[k])
    let held := values[0]
    let again := ref values[0]
}
fn M[...Types](values: ...ref mut Types) void {
    TouchMut(values[0])
    let r := ref mut values[0]
}
fn O[...Types](values: ...Types) void {
    let first := values[0]
    let view := ref values[0]
}
fn main() int {
    let a := 1
    let mut b := 2
    S(ref a, ref a)
    M(ref mut b)
    O(1)
    return 0
}
`), nil)
	for _, test := range []struct{ name, source, want string }{
		{"runtime", "fn F[...Types](i: int, values: ...ref Types) void {\n    Touch(values[i])\n}\n", "compile-time constant"},
		{"flow-known mutable", "fn F[...Types](values: ...ref Types) void {\n    let mut j := 0\n    Touch(values[j])\n}\n", "compile-time constant"},
		{"negative", "fn F[...Types](values: ...ref Types) void {\n    Touch(values[-1])\n}\n", "compile-time constant"},
		{"different positions", "fn F[...Types](values: ...ref Types) void {\n    Same(values[0], values[1])\n}\n", "cannot infer generic arguments for Same"},
		{"out of range", "fn F[...Types](values: ...ref Types) void {\n    Touch(values[2])\n}\nfn main() int {\n    let a := 1\n    F(ref a)\n    return 0\n}\n", "reads pack position 2, but this call passes 1 pack elements"},
		{"empty pack", "fn F[...Types](values: ...ref Types) void {\n    Touch(values[0])\n}\nfn main() int {\n    F()\n    return 0\n}\n", "reads pack position 0, but this call passes 0 pack elements"},
		{"move element", "fn F[...Types](values: ...Types) void {\n    let c :<- values[0]\n}\n", "cannot move element out of variadic parameter pack"},
		{"move ref mut holder", "fn F[...Types](values: ...ref mut Types) void {\n    let r := values[0]\n}\n", "cannot move element out of variadic parameter pack"},
		{"mutable through shared", "fn F[...Types](values: ...ref Types) void {\n    let r := ref mut values[0]\n}\n", "cannot create mutable reference to immutable place"},
		{"mutable through owned", "fn F[...Types](values: ...Types) void {\n    let r := ref mut values[0]\n}\n", "cannot create mutable reference to immutable place"},
		{"copy of move-only", "fn F[...Types](values: ...Types) void {\n    let c := values[0]\n}\nfn main() int {\n    let h := Handle { Id: 1 }\n    F(<-h)\n    return 0\n}\n", "position 0 is Handle, which is not implicitly copyable"},
		{"consuming element", "fn Take[T](->value: T) void {\n}\nfn F[...Types](values: ...Types) void {\n    Take(values[0])\n}\n", "Take"},
		{"return element", "fn F[...Types](values: ...ref Types) ref int {\n    return values[0]\n}\n", "must return ref int"},
		{"assign holder", "fn F[...Types](values: ...ref mut Types) void {\n    values[0] = values[0]\n}\n", "cannot mutate variadic parameter pack"},
	} {
		t.Run(test.name, func(t *testing.T) {
			found := false
			errors := analyzeSourceRaw(t, prelude+test.source)
			for _, err := range errors {
				found = found || strings.Contains(err.Error(), test.want)
			}
			if !found {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
	// An explicit borrow of a move-only element needs no copy.
	assertSemaErrors(t, analyzeSourceRaw(t, prelude+"fn F[...Types](values: ...Types) void {\n    let view := ref values[0]\n}\nfn main() int {\n    let h := Handle { Id: 1 }\n    F(<-h)\n    return 0\n}\n"), nil)
}

// ToString() exists on every parameter, pack element and interface value and
// resolves per specialization; a template cannot prove its effects, and it grants
// no other member and no format argument.
// Rules: rules/compiler/compiler_known_members.md "Generic lookup" (MD-122).
func TestUniversalToStringOnGenericValues(t *testing.T) {
	valid := `module main
interface Named {
    fn Name() int
}
interface Printable {
    fn ToString() Result[string, StringError]
}
type Point struct { X: int }
impl Point {
    fn ToString() Result[string, StringError] { return Ok("point") }
}
fn Plain[T](value: T) Result[string, StringError] { return value.ToString() }
fn Borrowed[T](value: ref T) Result[string, StringError] { return value.ToString() }
fn Constrained[T: Printable](value: ref T) Result[string, StringError] { return value.ToString() }
fn Failure[E: error](value: ref E) Result[string, StringError] { return value.ToString() }
fn View(value: ref Named) Result[string, StringError] { return value.ToString() }
fn First[...Types](values: ...Types) Result[string, StringError] { return values[0].ToString() }
fn Composite[...Types](index: uint, values: ...ref Types) Result[string, FormatError] {
    let mut position: uint := 0
    for ref value in values {
        if position == index {
            let text := try value.ToString() {
                Err(_) => {
                    return Err(FormatError.InvalidFormat)
                }
            }
            discard text
            break
        }
        position += 1
    }
    return Ok("")
}
fn Use() Result[string, StringError] {
    let number := 1
    let point := Point { X: 1 }
    discard Composite(0, number, point)
    return Plain(point)
}
`
	errors := analyzeSourceRaw(t, valid)
	for _, err := range errors {
		if err.Severity == "" || err.Severity == diagnostics.SeverityError {
			t.Errorf("%v", err)
		}
	}
	for _, item := range []struct{ source, want string }{
		{"@noPanic\nfn A[T](value: ref T) Result[string, StringError] { return value.ToString() }", "may-panic-unknown-callee"},
		{"@noAlloc\nfn A[T](value: ref T) Result[string, StringError] { return value.ToString() }", "unknown allocation behavior"},
		{"fn A[T](value: T) Result[string, StringError] { return value.ToString(1) }", "expects 0 arguments"},
		{"fn A[T](value: T) uint { return value.Len }", "Len"},
		{"fn A[T](value: T) string { return value.ToString() }", "must return string"},
	} {
		messages := ""
		for _, err := range analyzeSourceRaw(t, "module main\n"+item.source+"\n") {
			messages += err.Message + "\n"
		}
		if !strings.Contains(messages, item.want) {
			t.Errorf("%s: want %q, got %s", item.source, item.want, messages)
		}
	}
}
