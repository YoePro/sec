package sema

import (
	"strings"
	"testing"
)

const genericErrorPrelude = `module main
enum IOError error { Closed, Busy }
enum NetError error { Down }
interface Printer { fn P() int }
interface Sink[E: error] {
    mut fn Write(x: int) Result[void, E]
}
type S struct { N: int }
impl S implements Sink[IOError] {
    fn Write(x: int) Result[void, IOError] { return Ok() }
}
fn Forward[E: error](sink: ref mut Sink[E], x: int) Result[void, E] {
    try sink.Write(x)
    return Ok()
}
fn Ready[E: error]() Result[int, E] { return Ok(10) }
`

// MD-120 §§ 2–7, 11: the error category constraint, its template environment,
// generic error declarations, complete interface instantiations and inference
// without a fallback to error.
// Rules: rules/declarations/generics.md — §§ 11–15A, 18, 22–23;
// rules/errors/errorhandling.md — §§ 3.5, 4, 12.1, 17; rules/declarations/interfaces.md — §§ 7.1, 9.
func TestGenericErrorParameters(t *testing.T) {
	for name, source := range map[string]string{
		"explicit view":         "fn U() void {\n    let mut s := S { N: 1 }\n    let r := Forward[IOError](ref mut s, 1)\n}\n",
		"conformance inference": "fn U() void {\n    let mut s := S { N: 1 }\n    let r := Forward(ref mut s, 1)\n}\n",
		"infer from argument":   "fn W[E: error](cause: E) Result[void, E] { return Err(<-cause) }\nfn U() void { let r := W(IOError.Closed) }\n",
		"expected result":       "fn U() void { let r: Result[int, IOError] := Ready() }\n",
		"explicit argument":     "fn U() void { let r := Ready[IOError]() }\n",
		"expected error root":   "fn U() void { let r: Result[int, error] := Ready() }\n",
		"explicit error root":   "fn U() void { let r := Ready[error]() }\n",
		"same channel try":      "fn A[E: error](c: Result[int, E]) Result[int, E] {\n    let v := try c\n    return Ok(v)\n}\n",
		"widening try":          "fn A[E: error](c: Result[int, E]) Result[int, error] {\n    let v := try c\n    return Ok(v)\n}\n",
		"whole channel match":   "fn A[E: error](c: Result[int, E]) int {\n    match c {\n        Ok(v) => return v\n        Err(e) => return 0\n    }\n}\n",
		"conjunction":           "fn A[E: error & Printer](e: E) int { return e.P() }\n",
		"generic union":         "type W[E: error] union error { Format(int) Sink(E) }\nfn G[E: error](w: W[E]) int {\n    match w {\n        W.Format(n) => return n\n        W.Sink(e) => return 1\n    }\n}\nfn M() Result[int, W[IOError]] { return Err(W[IOError].Format(1)) }\n",
		"phantom enum":          "enum Op[T] error { Failed, Cancelled }\nfn A() Result[int, Op[int]] { return Err(Op[int].Failed) }\n",
		"non-error payload":     "type PE[T] union error { Bad(T) End }\nfn A() Result[int, PE[string]] { return Err(PE[string].End) }\n",
		"concrete Err(_)":       "fn A(c: Result[int, IOError]) int {\n    match c {\n        Ok(v) => return v\n        Err(_) => return 0\n    }\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			assertSemaErrors(t, analyzeSourceRaw(t, genericErrorPrelude+source), nil)
		})
	}
	for _, test := range []struct{ name, source, want string }{
		{"no fallback", "fn U() void { let r := Ready() }\n", "cannot infer generic arguments for Ready"},
		{"unconstrained", "fn R[E]() Result[void, E] { return Ok() }\n", "Result error type E is not an error type"},
		{"concrete constraint", "fn R[T: int](v: T) void {}\n", "generic constraint int is not an interface"},
		{"non-error argument", "fn B(s: ref mut Sink[string]) void {}\n", "type string does not satisfy constraint error for E"},
		{"bare interface", "fn B(s: ref mut Sink) void {}\n", "Sink requires 1 generic arguments, got 0"},
		{"no variance", "fn B(s: ref mut Sink[IOError]) void { C(s) }\nfn C(s: ref mut Sink[error]) void {}\n", "must be ref mut Sink[error]"},
		{"wrong explicit channel", "fn U() void {\n    let mut s := S { N: 1 }\n    let r := Forward[NetError](ref mut s, 1)\n}\n", "cannot infer remaining generic arguments"},
		{"contextual variant", "fn A[E: error](c: Result[int, E]) int {\n    match c {\n        Ok(v) => return v\n        Err(.Closed) => return 0\n    }\n}\n", "no variants to select"},
		{"named variant", "fn A[E: error](c: Result[int, E]) int {\n    match c {\n        Ok(v) => return v\n        Err(E.Closed) => return 0\n    }\n}\n", "no variants to select"},
		{"member", "fn A[E: error](e: E) void { e.Foo() }\n", "unknown function or type e.Foo"},
		{"copy", "fn A[E: error](e: E) E {\n    let c := e\n    return e\n}\n", "generic copyability has not been proven"},
		{"narrowing", "fn A[E: error](e: E) IOError { return e }\n", "must return IOError, got E"},
		{"root to parameter", "fn A[E: error](e: error) E { return e }\n", "must return E, got error"},
		{"unproved discard", "fn A[E: error](c: Result[int, E]) int {\n    match c {\n        Ok(v) => return v\n        Err(_) => return 0\n    }\n}\n", "Err(_) cannot ignore E"},
		{"cross channel", "fn A[E: error](c: Result[int, E]) Result[int, IOError] {\n    let v := try c\n    return Ok(v)\n}\n", "bodyless try propagates E"},
		{"unsatisfied conjunct", "fn A[E: error & Printer](e: E) int { return e.P() }\nfn U() void { let x := A(IOError.Closed) }\n", "does not satisfy constraint Printer for E"},
		{"non-error expected channel", "fn U() void { let r: Result[int, string] := Ready() }\n", "Result error type string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, genericErrorPrelude+test.source)
			found := false
			for _, err := range errors {
				found = found || strings.Contains(err.Error(), test.want)
			}
			if !found {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}
