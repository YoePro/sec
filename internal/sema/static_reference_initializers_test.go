package sema

import (
	"strings"
	"testing"
)

// MD-081: explicit `ref` / `ref mut` initializers of statics borrow verified
// static roots, their fields and constant-index elements, with typed or
// inferred destinations; ordinary borrow authority still applies.
// Rules: rules/declarations/static.md — §§ 5, 15 (Amendment MD-081);
// rules/corrections/applied/MD-081-static-reference-initializers-approved-20261030.md — §§ 2-6, 11.
func TestStaticReferenceInitializers(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

type Settings struct {
    Width: int,
    Height: int,
}

type Registry struct {
    id: int,
}

impl Registry {
    static let Limit: int := 8
    static let LimitRef := ref Registry.Limit
}

let Configuration: int := 42
let ConfigurationRef: ref int := ref Configuration
let ConfigurationRef2 := ref Configuration
let SharedTwice := ref Configuration

let mut Counter: int := 0
let CounterAccess := ref mut Counter

let mut Config: Settings := Settings{Width: 100, Height: 200}
let WidthAccess := ref mut Config.Width
let HeightAccess := ref mut Config.Height

let Table: int[3] := [1, 2, 3]
let Second := ref Table[1]
let Last := ref Table[1 + 1]
let Whole := ref Table

fn Persistent() int {
    static let Calls: int := 0
    static let CallsRef := ref Calls
    return 0
}
`)
	if len(errs) != 0 {
		t.Fatalf("valid static references rejected: %v", errs)
	}
}

// MD-081 § 12: implicit references, missing authority, overlapping exclusive
// borrows, out-of-bounds and runtime indexes and non-static roots are
// rejected with their specific diagnostics, never the generic runtime
// initializer error.
func TestStaticReferenceInitializerRejections(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

let Configuration: int := 42
let Implicit: ref int := Configuration
let Upgrade := ref mut Configuration

let mut Counter: int := 0
let First := ref mut Counter
let Second := ref mut Counter

let Table: int[3] := [1, 2, 3]
let OutOfBounds := ref Table[5]

fn Index() int {
    return 1
}

let RuntimeIndex := ref Table[Index()]

fn Local() int {
    let automatic: int := 3
    static let Held := ref automatic
    return 0
}
`)
	wants := []string{
		"ref int is required here, got owned int",
		"cannot create mutable reference to immutable variable Configuration",
		"cannot create mutable reference to Counter while it is already borrowed",
		"array index 5 is out of bounds for int[3]",
		"static reference initializer for RuntimeIndex needs a compile-time index",
		"static reference initializer for Held borrows automatic, which is not static storage",
	}
	joined := ""
	for _, err := range errs {
		joined += err.Message + "\n"
		if strings.Contains(err.Message, "must be compile-time evaluable") {
			t.Errorf("generic runtime-initializer error instead of a specific one: %v", err)
		}
	}
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
}

// MD-081 § 6(3): a borrow held by a static reference holder stays live in
// every callable, so direct access that conflicts with it is rejected in any
// declaration order, while use through the holder and shared reads beside a
// shared static reference stay valid.
func TestStaticReferenceHoldersStayLiveInCallables(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

fn Bump() void {
    Counter = Counter + 1
}

fn Take(value: ref mut int) void {
}

fn UseHolder() void {
    Take(CounterAccess)
}

let mut Counter: int := 0
let CounterAccess := ref mut Counter

let Shared: int := 5
let SharedRef := ref Shared

fn ReadShared() int {
    return Shared
}
`)
	if len(errs) != 1 || errs[0].Line != 5 || !strings.Contains(errs[0].Message, "cannot assign to Counter while it is mutably borrowed") {
		t.Fatalf("errors = %v", errs)
	}
}

// MD-081 § 6(3): a function-local static reference holder keeps its borrow
// across invocations, so conflicting access in any callable, including its
// own function before the declaration, is rejected; shared holders allow
// reads.
func TestFunctionLocalStaticReferenceHolders(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

let mut Counter: int := 0
let Shared: int := 1

fn Elsewhere() void {
    Counter = 2
}

fn Owner() int {
    let before: int := Counter
    static let Held := ref mut Counter
    static let SharedHeld := ref Shared
    return before + Shared
}

fn Reader() int {
    return Shared
}
`)
	wants := []struct {
		line    int
		message string
	}{
		{8, "cannot assign to Counter while it is mutably borrowed"},
		{12, "cannot read Counter while it is mutably borrowed"},
	}
	if len(errs) != len(wants) {
		t.Fatalf("errors = %v", errs)
	}
	for i, want := range wants {
		if errs[i].Line != want.line || !strings.Contains(errs[i].Message, want.message) {
			t.Errorf("error %d = %v, want %q at %d", i, errs[i], want.message, want.line)
		}
	}
}

// MD-081 §§ 7, 12(2), 12(4): a borrowed static cannot be replaced or moved,
// and a computed value is reported once as non-addressable (S1168).
func TestStaticReferenceReferentStaysValid(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }

    fn Get() int {
        return self.id
    }
}

let mut Resource: Handle := Handle{id: 1}
let ResourceRef := ref Resource
let Computed := ref Resource.Get()

fn Consume(h: Handle) void {
}

fn Replace() void {
    Resource = Handle{id: 2}
}

fn Move() void {
    Consume(<-Resource)
}
`)
	wants := []string{
		"cannot borrow temporary expression; a reusable place is required",
		"cannot assign to Resource while it is shared borrowed",
		"cannot move Resource while it is borrowed",
	}
	if len(errs) != len(wants) {
		t.Fatalf("errors = %v", errs)
	}
	for i, want := range wants {
		if !strings.Contains(errs[i].Message, want) {
			t.Errorf("error %d = %v, want %q", i, errs[i], want)
		}
	}
}
