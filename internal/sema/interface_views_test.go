package sema

import (
	"os"
	"strings"
	"testing"
)

// Borrowed interface views (MD-057): a concrete type with explicit
// conformance is passed to `ref I` / `ref mut I` without a generic parameter
// or dyn, two calls may pass different concrete types, a `ref mut` view calls
// a mut fn, and `for` iterates a `ref mut Iterator[T]` view with a resolved
// plan whose source is the borrowed view (correction §§ 8.1–8.4).
//
// Rules:
//   - rules/declarations/interfaces.md — § 7.1
//   - rules/control-flow/flowcontrol_for.md — § 37.1
//   - rules/corrections/applied/md057-interface-iterator-parameters-correction-20261017.md — § 8
func TestBorrowedInterfaceViewsAcceptConformingArguments(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_views_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	views := 0
	for _, plan := range analyzer.resolvedForIterations {
		if plan.Source == ForIteratorBorrowedView {
			views++
			if plan.ElementType.Kind != IntType || !isIteratorInterface(plan.Conformance) {
				t.Fatalf("view plan = %+v", plan)
			}
		}
	}
	if views != 1 {
		t.Fatalf("borrowed iterator view plans = %d", views)
	}
}

// The negative half of the matrix: shared authority never reaches a mut fn
// or a ref mut view, matching member names without `implements` do not
// conform, a shared ref Iterator[T] is not an iteration source, and a by-value
// Iterator[T] parameter is rejected with S1172 instead of becoming an owned
// value or an implicit generic (correction §§ 8.2, 8.6, 8.7, 8.9).
func TestBorrowedInterfaceViewsRejectInvalidUses(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_views_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	for _, want := range []string{
		"method Increment requires mutable receiver",
		"requires ref mut Iterator[int]; a shared ref cannot call mut fn Next",
		"cannot create mutable reference to immutable variable tally",
		"argument 1 to Bump must be ref mut Counter, got ref Tally",
		"argument 1 to Drain must be ref mut Iterator[int], got ref mut Lookalike",
		"parameter \"values\" cannot have interface type Iterator[int] by value",
	} {
		found := false
		for _, err := range errors {
			found = found || strings.Contains(err.Error(), want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errors)
		}
	}
	for _, err := range errors {
		if strings.Contains(err.Error(), "cannot have interface type") && err.ID != "S1172" {
			t.Errorf("by-value Iterator parameter not S1172: %v", err)
		}
	}
}

// Forwarding a type pack accepts the same borrowed interface views as a call without
// a spread, for an ordinary argument and for a pack position, and keeps rejecting a
// non-conforming type and a shared reference where the view is mutable.
// Rules: rules/declarations/interfaces.md — §§ 7.1, 9; rules/declarations/functions.md — § 38A.
func TestPackForwardingAcceptsBorrowedInterfaceViews(t *testing.T) {
	const declarations = `module main

interface Sink[E: error] {
    fn Put() Result[void, E]
}

type Failure enum error { Broken }

type Buffer struct {
    count: int,
}

impl Buffer implements Sink[Failure] {
    fn Put() Result[void, Failure] {
        return Ok()
    }
}

type Plain struct {
    count: int,
}

fn Compose[E: error, ...Types](sink: ref mut Sink[E], values: ...ref Types) Result[void, E] {
    return sink.Put()
}

fn Take(sink: ref Sink[Failure]) Result[void, Failure] {
    return Ok()
}

fn TakeMut(sink: ref mut Sink[Failure]) Result[void, Failure] {
    return Ok()
}

fn Exact(sink: ref Sink[Failure]) int {
    return 0
}

fn Exact(sink: ref Buffer) int {
    return 1
}
`
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"local argument": {source: `
fn F[...Types](values: ...ref Types) Result[void, Failure] {
    let mut buffer := Buffer { count: 0 }
    return Compose(ref mut buffer, values...)
}

fn T() Result[void, Failure] {
    let number := 1
    return F(ref number)
}
`},
		"reborrowed argument": {source: `
fn F[...Types](sink: ref mut Buffer, values: ...ref Types) Result[void, Failure] {
    return Compose(ref mut sink, values...)
}

fn T() Result[void, Failure] {
    let mut buffer := Buffer { count: 0 }
    let number := 1
    return F(ref mut buffer, ref number)
}
`},
		"pack position": {source: `
fn G[...Types](values: ...ref Types) Result[void, Failure] {
    return Take(values...)
}

fn T() Result[void, Failure] {
    let buffer := Buffer { count: 0 }
    return G(ref buffer)
}
`},
		"exact overload still ranks first": {source: `
fn G[...Types](values: ...ref Types) int {
    return Exact(values...)
}

fn T() int {
    let buffer := Buffer { count: 0 }
    return G(ref buffer)
}
`},
		"non-conforming position": {source: `
fn G[...Types](values: ...ref Types) Result[void, Failure] {
    return Take(values...)
}

fn T() Result[void, Failure] {
    let plain := Plain { count: 0 }
    return G(ref plain)
}
`, want: "cannot be passed to parameter"},
		"shared position to mutable view": {source: `
fn G[...Types](values: ...ref Types) Result[void, Failure] {
    return TakeMut(values...)
}

fn T() Result[void, Failure] {
    let buffer := Buffer { count: 0 }
    return G(ref buffer)
}
`, want: "a shared reference never grants mutation"},
	} {
		t.Run(name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzerRaw(t, declarations+test.source)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatal(errors)
				}
				return
			}
			found := false
			for _, problem := range errors {
				found = found || strings.Contains(problem.Message, test.want)
			}
			if !found {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}

// A generic parameter constrained by an interface is a concrete type with that
// explicit conformance in every specialization, so it passes as a borrowed view.
// Rules: rules/declarations/interfaces.md — §§ 7.1, 9; rules/declarations/generics.md — § 14.
func TestConstrainedGenericPassesAsBorrowedView(t *testing.T) {
	const declarations = `module main

interface Sink {
    fn Put() int
}

interface Child implements Sink {
    fn Extra() int
}

interface Other {
    fn Skip() int
}

interface Typed[E: error] {
    fn Fail() Result[void, E]
}

type Failure enum error { Broken }

type Buffer struct {
    count: int,
}

impl Buffer implements Sink, Typed[Failure] {
    fn Put() int {
        return 1
    }

    fn Fail() Result[void, Failure] {
        return Ok()
    }
}

fn View(sink: ref mut Sink) int {
    return sink.Put()
}

fn Shared(sink: ref Sink) int {
    return sink.Put()
}

fn TypedView[E: error](sink: ref mut Typed[E]) Result[void, E] {
    return sink.Fail()
}

fn Compose[...Types](sink: ref mut Sink, values: ...ref Types) int {
    return sink.Put()
}
`
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"holder":    {source: "fn F[W: Sink](d: ref mut W) int { return View(d) }"},
		"reborrow":  {source: "fn F[W: Sink](d: ref mut W) int { return View(ref mut d) }"},
		"shared":    {source: "fn F[W: Sink](d: ref W) int { return Shared(d) }"},
		"to shared": {source: "fn F[W: Sink](d: ref mut W) int { return Shared(d) }"},
		"child":     {source: "fn F[W: Child](d: ref mut W) int { return View(d) }"},
		"specialized": {source: `fn F[W: Sink](d: ref mut W) int { return View(d) }

fn T() int {
    let mut buffer := Buffer { count: 0 }
    return F(ref mut buffer)
}`},
		"generic interface": {source: "fn F[E: error, W: Typed[E]](d: ref mut W) Result[void, E] { return TypedView(d) }"},
		"pack forwarding":   {source: "fn F[W: Sink, ...Types](d: ref mut W, values: ...ref Types) int { return Compose(d, values...) }"},
		"unconstrained":     {source: "fn F[W](d: ref mut W) int { return View(d) }", want: "must be ref mut Sink"},
		"unrelated":         {source: "fn F[W: Other](d: ref mut W) int { return View(d) }", want: "must be ref mut Sink"},
		"shared to mutable": {source: "fn F[W: Sink](d: ref W) int { return View(d) }", want: "must be ref mut Sink"},
	} {
		t.Run(name, func(t *testing.T) {
			_, errors := analyzeSourceWithAnalyzerRaw(t, declarations+"\n"+test.source+"\n")
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatal(errors)
				}
				return
			}
			found := false
			for _, problem := range errors {
				found = found || strings.Contains(problem.Message, test.want)
			}
			if !found {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}
