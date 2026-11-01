package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func spreadFixtureMessages(t *testing.T, source string) []string {
	t.Helper()
	messages := []string{}
	for _, err := range analyzeSourceRaw(t, source) {
		if err.Severity == "" || err.Severity == diagnostics.SeverityError {
			messages = append(messages, err.Message)
		}
	}
	return messages
}

const spreadTargets = `module main
fn Take[...T](values: ...T) uint { return values.Len }
fn Read[...T](values: ...ref T) uint { return values.Len }
fn Write[...T](values: ...ref mut T) uint { return values.Len }
fn Two(a: int, b: string) int { return a }
fn Sum(values: ...int) int { return 0 }
fn Scan[...T](values: ...T) uint {
    let mut count: uint := 0
    for value in values {
        count = count + 1
    }
    return count
}
`

// The template checks of pack forwarding: `<-values...` is one whole-pack
// transfer of an owned pack, authority never grows and a ref mut is never copied.
// Rules: rules/declarations/functions.md §38A(7); rules/declarations/spread.md §4.4.
func TestTypePackForwardTemplate(t *testing.T) {
	valid := []string{
		"fn A[...T](values: ...T) uint { return Take(values...) }",
		"fn A[...T](values: ...T) uint { return Take(<-values...) }",
		"fn A[...T](values: ...T) uint { return Read(values...) }",
		"fn A[...T](values: ...ref T) uint { return Read(values...) }",
		"fn A[...T](values: ...ref mut T) uint { return Write(values...) }",
		"fn A[...T](values: ...ref mut T) uint { return Read(values...) }",
		"fn A[...T](values: ...T) uint { return Take(1, values...) }",
		"fn A[...T](values: ...T) uint { return Take(values..., 1) }",
		"fn A[...T](x: ref int, values: ...ref T) uint { return Read(x, values..., x) }",
		"fn A[...T](values: ...ref T) uint { return Take(values...) }",
		"fn A[...T](values: ...ref mut T) uint { return Take(values...) }",
		"fn A[...T](values: ...T) uint { return Take(values..., values...) }",
		"fn A[...T](flag: bool, values: ...T) uint {\n if flag { return Take(<-values...) }\n return values.Len\n}",
		"fn A(values: ...int) int { return Sum(<-values...) }",
		"fn A(values: ...int) int { return Sum(values...) }",
	}
	for _, body := range valid {
		if messages := spreadFixtureMessages(t, spreadTargets+body+"\n"); len(messages) != 0 {
			t.Errorf("%s: %v", body, messages)
		}
	}
	invalid := []struct{ body, message string }{
		{"fn A[...T](values: ...ref T) uint { return Take(<-values...) }", "needs a currently owned pack"},
		{"fn A[...T](values: ...ref mut T) uint { return Take(<-values...) }", "needs a currently owned pack"},
		{"fn A[...T](values: ...T) uint { return Read(<-values...) }", "moves the pack"},
		{"fn A[...T](values: ...T) uint { return Write(values...) }", "is read-only"},
		{"fn A[...T](values: ...ref T) uint { return Write(values...) }", "never grants mutation"},
		{"fn One(a: int) uint { return 0 }\nfn A[...T](values: ...ref T) uint { return One(values...) }\nfn M() uint {\n let a := 1\n return A(a)\n}", "never dereferenced implicitly"},
		{"fn A[...T](values: ...ref mut T) uint { return Write(values..., values...) }", "expanded twice"},
		{"fn A[...T](values: ...T) uint { return Take(<-values..., values...) }", "expanded twice"},
		{"fn A[...T](values: ...T) uint { return Take(<-values[0]) }", "cannot move element"},
		{"fn A[...T](values: ...T) uint { return Take(<-values) }", "semantic expansion"},
		{"fn A[...T](values: ...T) uint {\n let a := Take(<-values...)\n return values.Len\n}", "no longer available"},
		{"fn A[...T](values: ...T) uint {\n let a := Take(<-values...)\n return Take(<-values...)\n}", "no longer available"},
		{"fn A[...T](flag: bool, values: ...T) uint {\n if flag { let a := Take(<-values...) }\n return values.Len\n}", "may no longer be available"},
		{"fn A[...T](values: ...T) uint {\n for value in values { let a := Take(<-values...) }\n return 0\n}", "later loop iteration"},
		{"fn A[...T](values: ...ref mut T) uint {\n let held := ref mut values[0]\n let n := Write(values...)\n held\n return n\n}", "mutably borrowed"},
		{"fn A[...T](values: ...ref mut T) uint { return Write(ref mut values[0], values...) }", "expands the complete pack"},
		{"fn A(values: int[2]) uint { return Take(<-values...) }", "needs a complete, currently owned"},
		{"fn A(values: int[]) uint { return Take(<-values...) }", "needs a complete, currently owned"},
		{"fn A(values: ...int) int { return Sum(<-values..., values...) }", "moves the complete pack"},
		{"fn A(values: ...int) int {\n let a := Sum(<-values...)\n return Sum(values...)\n}", "no longer available"},
		{"fn A[...T](values: ...T) void { let list := [values...] }", "semantic expansion"},
		{"fn A[...T](values: ...T) uint {\n let f := fn() uint { return Take(values...) }\n return f()\n}", "semantic expansion"},
		{"fn F(a: int) int { return 0 }\nfn F(a: string) bool { return true }\nfn A[...T](values: ...T) void { discard F(values...) }", "different result types"},
		{"fn F(a: ref int, b: int) void {}\nfn F(a: int, b: string) void {}\nfn A[...T](x: int, values: ...T) void { F(x, values...) }", "different modes for an ordinary argument"},
		{"fn P[V, ...R](a: V, rest: ...R) V { return a }\nfn A[...T](values: ...T) int { return P(values...) }", "result type depends on the pack"},
		{"type V struct { Item: ref int }\nfn Mk(a: ref int) V { return V { Item: a } }\nfn A[...T](values: ...ref T) V { return Mk(values...) }", "contains a reference"},
		{"fn Two2(a: int, b: int[]) int { return 0 }\nfn A[...T](data: int[], values: ...T) int { return Two2(values..., <-data) }", "must move or borrow"},
		{"fn A[...T](values: ...T) uint { let a: int[2] := [1, 2]\n return Take(a..., values...) }", "array or slice spread"},
	}
	for _, item := range invalid {
		messages := spreadFixtureMessages(t, spreadTargets+item.body+"\n")
		if !strings.Contains(strings.Join(messages, "\n"), item.message) {
			t.Errorf("%s: want %q, got %v", item.body, item.message, messages)
		}
	}
}

// Arity, position types and copyability exist per specialization, so each
// selected sequence proves its forwards, through chains, and reports at the
// source call. Rules: rules/declarations/functions.md §38A(4)(2),(4)(6),(10)(1–2).
func TestTypePackForwardSpecializations(t *testing.T) {
	cases := []struct{ body, message string }{
		{"fn A[...T](values: ...T) int { return Two(values...) }\nfn M() int { return A(1) }", "expands to 1"},
		{"fn A[...T](values: ...T) int { return Two(values...) }\nfn M() int { return A(1, 2) }", "pack position 1 of type int cannot be passed to parameter b"},
		{"fn A[...T](values: ...T) int { return Sum(values...) }\nfn M() int { return A(\"x\") }", "pack position 0 of type string"},
		{"fn A[...T](values: ...T) uint { return Take(values...) }\nfn M() uint {\n let data: int[] := []\n return A(<-data)\n}", "pack position 0 of type int[] is not implicitly copyable"},
		{"fn N[...T](values: ...T) uint { return Scan(<-values...) }\nfn M() uint {\n let data: int[] := []\n return N(<-data)\n}", "Scan copies pack elements by value"},
		{"fn G[V](a: V, b: V) void {}\nfn A[...T](values: ...T) void { G(values...) }\nfn M() void { A(1, true) }", "cannot be instantiated"},
		{"fn G[...T](values: ...T) uint { return G(1, values...) }\nfn M() uint { return G() }", "exceeds the limit"},
	}
	for _, item := range cases {
		messages := spreadFixtureMessages(t, spreadTargets+item.body+"\n")
		if !strings.Contains(strings.Join(messages, "\n"), item.message) {
			t.Errorf("%s: want %q, got %v", item.body, item.message, messages)
		}
	}
	accepted := []string{
		"fn A[...T](values: ...T) int { return Two(values...) }\nfn M() int { return A(1, \"x\") }",
		"fn N[...T](values: ...T) uint { return Take(<-values...) }\nfn M() uint {\n let data: int[] := []\n return N(<-data)\n}",
		"fn N[...T](values: ...T) uint { return Read(values...) }\nfn M() uint {\n let data: int[] := []\n return N(<-data)\n}",
		"fn G[V](a: V, b: V) void {}\nfn A[...T](values: ...T) void { G(values...) }\nfn M() void { A(1, 2) }",
		"fn P[V, ...R](a: V, rest: ...R) V { return a }\nfn A[...T](values: ...T) int { return P(1, values...) }\nfn M() int { return A(true) }",
	}
	for _, body := range accepted {
		if messages := spreadFixtureMessages(t, spreadTargets+body+"\n"); len(messages) != 0 {
			t.Errorf("%s: %v", body, messages)
		}
	}
}

const spreadMethods = `module main
type Box[V] struct { Value: V }
type Counter struct { Total: uint }
impl Box[V] {
    fn Size[...T](values: ...T) uint { return values.Len }
    fn Keep(first: V) V { return first }
}
impl Counter {
    fn Count[...T](values: ...ref T) uint { return values.Len }
    fn Write[...T](values: ...ref mut T) uint {
        self.Total += values.Len
        return self.Total
    }
    fn Two(a: int, b: bool) uint { return 2 }
    static fn Make[...T](values: ...T) uint { return values.Len }
    fn Inner[...T](values: ...ref T) uint { return Count(values...) }
}
fn Pick(a: int) int { return 1 }
fn Pick(a: bool) int { return 2 }
fn Pick(a: int, b: int) int { return 3 }
fn Ambiguous(a: int) int { return 1 }
fn Ambiguous[T](a: T) int { return 2 }
fn Variadic(a: int) int { return 1 }
fn Variadic(values: ...int) int { return 2 }
`

// Methods take the same receiver checks as a direct call; overloads are selected
// per specialization by ordinary ranking, and only the result is fixed in the template.
// Rules: rules/declarations/functions.md §§22–24,37,38A(4)(5),(7).
func TestTypePackForwardMethodsAndOverloads(t *testing.T) {
	for _, body := range []string{
		"fn A[...T](counter: Counter, values: ...ref T) uint { return counter.Count(values...) }\nfn M() uint {\n let c := Counter { Total: 0 }\n let a := 1\n return A(c, a)\n}",
		"fn A[...T](box: Box[int], values: ...T) uint { return box.Size(values...) }\nfn M() uint {\n let b := Box[int] { Value: 1 }\n return A(b, true)\n}",
		"fn A[...T](box: Box[int], values: ...T) int { return box.Keep(values...) }\nfn M() int {\n let b := Box[int] { Value: 1 }\n return A(b, 1)\n}",
		"fn A[...T](values: ...T) uint { return Counter.Make(values...) }\nfn M() uint { return A(1) }",
		"fn A[...T](values: ...T) int { return Pick(values...) }\nfn M() int { return A(1) + A(true) + A(1, 2) }",
		"fn A[...T](values: ...T) int { return Variadic(values...) }\nfn M() int { return A(1) + A(1, 2) }",
	} {
		if messages := spreadFixtureMessages(t, spreadMethods+body+"\n"); len(messages) != 0 {
			t.Errorf("%s: %v", body, messages)
		}
	}
	for _, item := range []struct{ body, want string }{
		{"fn A[...T](box: Box[int], values: ...T) int { return box.Keep(values...) }\nfn M() int {\n let b := Box[int] { Value: 1 }\n return A(b, true)\n}", "pack position 0 of type bool cannot be passed to parameter first of type int"},
		{"fn A[...T](counter: ref Counter, values: ...ref mut T) uint { return counter.Write(values...) }", "mutable receiver"},
		{"fn A[...T](values: ...T) int { return Ambiguous(values...) }\nfn M() int { return A(1) }", "ambiguous call to Ambiguous"},
		{"fn A[...T](values: ...T) uint { return Counter.Count(values...) }", "requires an instance receiver"},
		{"fn A[...T](values: ...T) int { return Pick(values...) }\nfn M() int { return A(\"x\") }", "no overload of Pick accepts"},
		{"fn A[...T](values: ...T) int { return Pick(values...) }\nfn M() int { return A() }", "no overload of Pick accepts"},
		{"fn A[...T](values: ...T) int { return Pick(<-values...) }\nfn M() int {\n let d: int[] := []\n return A(<-d)\n}", "no overload of Pick accepts"},
		{"interface Named { fn Name() int }\nfn A[...T](n: Named, values: ...T) int { return n.Name(values...) }", "through interface dispatch"},
	} {
		messages := spreadFixtureMessages(t, spreadMethods+item.body+"\n")
		if !strings.Contains(strings.Join(messages, "\n"), item.want) {
			t.Errorf("%s: want %q, got %v", item.body, item.want, messages)
		}
	}
}

const spreadReturns = `module main
fn Pick(a: ref int, b: ref bool) ref int { return a }
fn PickMut(a: ref mut int, b: ref mut bool) ref mut int { return a }
fn Second(a: ref bool, b: ref int) ref int { return b }
fn F[...T](values: ...ref T) ref int { return Pick(values...) }
fn W[...T](values: ...ref mut T) ref mut int { return PickMut(values...) }
fn Late[...T](values: ...ref T) ref int { return Second(values...) }
fn Fixed[...T](x: ref int, values: ...ref T) ref int { return x }
`

// A returned reference carries a finite caller-side origin set over every expanded
// position, an exclusive grant stays live over all of them, and an owned pack's
// storage never escapes. A leading parameter stays exact.
// Rules: rules/declarations/functions.md §38A(8); correction MD-119 §5.
func TestTypePackReturnedReferenceOrigins(t *testing.T) {
	for _, body := range []string{
		"fn M(p: ref int, q: ref bool) ref int { return F(p, q) }",
		"fn M() int {\n let a := 1\n let b := true\n let r := F(a, b)\n return *r\n}",
		"fn M(p: ref int) ref int {\n let b := true\n return Fixed(p, b)\n}",
		"fn O[...T](values: ...T) int {\n let r := Pick(values...)\n return *r\n}\nfn M() int { return O(1, true) }",
		"fn G[V, ...R](first: ref V, rest: ...ref R) ref V { return first }\nfn A[...T](x: ref int, values: ...ref T) ref int { return G(x, values...) }\nfn M(p: ref int) ref int {\n let b := true\n return A(p, b)\n}",
		"fn H[...T](values: ...ref T) ref int {\n let r := Pick(values...)\n return r\n}\nfn M(p: ref int, q: ref bool) ref int { return H(p, q) }",
	} {
		if messages := spreadFixtureMessages(t, spreadReturns+body+"\n"); len(messages) != 0 {
			t.Errorf("%s: %v", body, messages)
		}
	}
	for _, item := range []struct{ body, want string }{
		{"fn M() ref int {\n let a := 1\n let b := true\n return F(a, b)\n}", "reference to local variable a"},
		{"fn M(p: ref int) ref int {\n let b := true\n return F(p, b)\n}", "reference to local variable b"},
		{"fn M(q: ref bool) ref int {\n let b := 1\n return Late(q, b)\n}", "reference to local variable b"},
		{"fn O[...T](values: ...T) ref int { return Pick(values...) }", "reference to local variable values"},
		{"fn M() int {\n let mut a := 1\n let mut b := true\n let r := W(a, b)\n let c := a\n *r = 2\n return c\n}", "cannot read a while it is mutably borrowed"},
		{"fn M() bool {\n let mut a := 1\n let mut b := true\n let r := W(a, b)\n let c := b\n *r = 2\n return c\n}", "cannot read b while it is mutably borrowed"},
		{"fn M() int {\n let mut b := 2\n let t := true\n let r := Late(t, b)\n b = 3\n return *r\n}", "cannot assign to b while it is shared borrowed"},
		{"fn R[...T](values: ...ref T) ref int { return R(values...) }", "unknown control-flow provenance"},
		{"fn G[...T](values: ...ref T) ref int { return values[0] }", "must return ref int"},
	} {
		messages := spreadFixtureMessages(t, spreadReturns+item.body+"\n")
		if !strings.Contains(strings.Join(messages, "\n"), item.want) {
			t.Errorf("%s: want %q, got %v", item.body, item.want, messages)
		}
	}
}

// The canonical valid and invalid spread families of rules/declarations/spread.md §14.
func TestSpreadCanonicalFixtures(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/sema/spread_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if messages := spreadFixtureMessages(t, string(valid)); len(messages) != 0 {
		t.Fatal(messages)
	}
	invalid, err := os.ReadFile("../../testdata/sema/spread_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	messages := strings.Join(spreadFixtureMessages(t, string(invalid)), "\n")
	for _, want := range []string{
		"cannot spread int into fixed-arity call",
		"cannot spread int[] into fixed-arity call",
		"function Two expects 2 arguments, got 3",
		"argument 1 to Words must be string, got int",
		"cannot spread Handle[2] into function arguments; indexed expansion would require unsupported consuming element reads",
		"cannot spread int[] into fixed-array literal",
		"array element 1 must be int, got bool",
		"array literal has 2 elements, expected 1",
		"cannot spread Other into Point",
		`field "Value" in struct Required has no default value`,
		`duplicate field "X"`,
		"cannot spread Holder into Holder",
		"<-values... needs a complete, currently owned native parameter pack; int[2]",
		"<-values... needs a complete, currently owned native parameter pack; int[]",
		"spread operator is not valid as a standalone expression",
	} {
		if !strings.Contains(messages, want) {
			t.Errorf("want %q in:\n%s", want, messages)
		}
	}
}
