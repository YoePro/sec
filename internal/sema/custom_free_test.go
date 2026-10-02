package sema

import (
	"fmt"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func errorAt(err Error) string {
	return fmt.Sprintf("%s at %d:%d", err.Message, err.Line, err.Column)
}

const customFreeTypes = `
module main

type Handle struct {
    raw: int,
}

impl Handle {
    free {
        Release(self.raw)
        self.raw = 0
    }
}

@noCopy
type Resource struct {
    raw: int,
}

type ResourcePair struct {
    First: Resource,
    Second: Resource,
}

impl ResourcePair {
    free {
        Release(self.First.raw)
    }
}

type Holder struct {
    pair: ResourcePair,
}

fn Release(raw: int) void {
}
`

// A free body is analyzed as the type's non-callable lifecycle member with
// exclusive authority over self; it may read and update fields, and a type
// may declare it in an extending impl block.
//
// Rules:
//   - rules/declarations/impl.md — §19 "`free`"
//   - rules/memory/destruction.md — §15.2 "Lifecycle status", §15.3 "`self` during `free`"
func TestCustomFreeBodyIsAnalyzedAsLifecycleMember(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Handle struct {
    raw: int,
}

impl Handle {
    fn Raw() int {
        return self.raw
    }
}

impl extends Handle {
    free {
        Release(self.Raw())
        self.raw = 0
        let local := Handle { raw: 1 }
        discard local
    }
}

fn Release(raw: int) void {
}

fn Use(handle: Handle) int {
    return handle.Raw()
}
`)
	assertSemaErrors(t, errors, nil)
}

// Partial moves are rejected for every value whose type defines custom free:
// ordinary locals, nested holders, ordinary methods, and self inside free.
// Moving the complete value remains valid.
//
// Rules:
//   - rules/memory/ownership.md — §19 "Custom `free` forbids partial moves in Sec 0.1"
//   - rules/memory/copy_move.md — §14(4)
//   - rules/memory/destruction.md — §10.3 "Custom `free`", §15.4(3)
func TestCustomFreeForbidsPartialMoves(t *testing.T) {
	errors := analyzeSourceRaw(t, customFreeTypes+`
impl extends ResourcePair {
    fn Take() void {
        let first :<- self.First
        discard first
    }
}

fn Split(pair: ResourcePair) void {
    let first :<- pair.First
    discard first
}

fn Nested(holder: Holder) void {
    let second :<- holder.pair.Second
    discard second
}

fn Whole(holder: Holder) void {
    let pair :<- holder.pair
    discard pair
}

fn MoveAll(pair: ResourcePair) void {
    let whole :<- pair
    discard whole
}
`)
	got := []string{}
	for _, err := range errors {
		got = append(got, errorAt(err))
	}
	assertStrings(t, got, []string{
		"cannot move pair.First out of pair because type ResourcePair defines custom free; partial moves are not permitted from custom-free types at 46:23",
		"cannot move holder.pair.Second out of holder.pair because type ResourcePair defines custom free; partial moves are not permitted from custom-free types at 51:31",
		"cannot move self.First out of self because type ResourcePair defines custom free; partial moves are not permitted from custom-free types at 40:27",
	})
	for _, err := range errors {
		if err.ID != "S1085" || err.PreviousLine != 26 {
			t.Fatalf("partial-move diagnostic = %+v, want S1085 related to free at 26", err)
		}
	}
}

// free may not register defer, return a value, or consume complete self, and
// self fields stay non-movable inside it.
//
// Rules:
//   - rules/memory/destruction.md — §15.3(2), §15.4(3), §15.6 "`defer` inside `free`", §15.7(1)
func TestCustomFreeBodyRestrictions(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

@noCopy
type Resource struct {
    raw: int,
}

type Owner struct {
    inner: Resource,
}

impl Owner {
    free {
        defer {
            Release(1)
        }
        let inner :<- self.inner
        discard inner
        Consume(<-self)
        discard self
        return 1
    }
}

fn Release(raw: int) void {
}

fn Consume(-> owner: Owner) void {
    discard owner
}
`)
	want := map[string]string{
		"S1086": "defer is not allowed inside free at 15:9",
		"S1085": "cannot move self.inner out of self because type Owner defines custom free; partial moves are not permitted from custom-free types at 18:27",
		"S1087": "free cannot consume, transfer, or resurrect complete self at 20:17",
	}
	seen := map[string]bool{}
	sawReturn := false
	for _, err := range errors {
		if message, ok := want[err.ID]; ok && errorAt(err) == message {
			seen[err.ID] = true
		}
		if err.Line == 22 {
			sawReturn = true
		}
	}
	for id, message := range want {
		if !seen[id] {
			t.Errorf("missing %s %q in %v", id, message, errors)
		}
	}
	if !sawReturn {
		t.Errorf("return with a value inside free was accepted: %v", errors)
	}
	selfDiscards := 0
	for _, err := range errors {
		if err.ID == "S1087" {
			selfDiscards++
		}
	}
	if selfDiscards != 2 {
		t.Errorf("S1087 count = %d, want consuming call and discard of self: %v", selfDiscards, errors)
	}
}

// At most one free exists for the merged implementation of a type, and only
// concrete types declare one.
//
// Rules:
//   - rules/declarations/impl.md — §19 "`free`"
func TestCustomFreeDeclarationValidity(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Handle struct {
    raw: int,
}

impl Handle {
    free {
    }
}

impl extends Handle {
    free {
    }
}

interface Closer {
    fn Close() void
}

impl Closer {
    free {
    }
}
`)
	ids := []string{}
	for _, err := range errors {
		if err.ID == "S1084" {
			ids = append(ids, errorAt(err))
		}
	}
	want := []string{
		"type Handle already declares free; a type has at most one free across its primary and extending impl blocks at 14:5",
		"interface Closer cannot declare free; free belongs to a concrete owning type at 23:5",
	}
	if len(ids) != len(want) {
		t.Fatalf("S1084 diagnostics = %v, want %v (all: %v)", ids, want, errors)
	}
	for index := range want {
		if ids[index] != want[index] {
			t.Fatalf("S1084[%d] = %q, want %q", index, ids[index], want[index])
		}
	}
}

// A type with custom free is non-trivially destructible even when its fields
// are trivial, and that classification propagates through aggregates and
// concrete generic instances.
//
// Rules:
//   - rules/memory/destruction.md — §3.3(2) "Non-trivially destructible"
func TestCustomFreeTypesAreNonTriviallyDestructible(t *testing.T) {
	program := parser.New(lexer.New(customFreeTypes + `
type Box[T] struct {
    value: T,
}

impl Box[T] {
    free {
    }
}

type Plain struct {
    raw: int,
}

fn Use(handle: Handle, holder: Holder, boxed: Box[int], plain: Plain) void {
    discard handle
    discard holder
    discard boxed
    discard plain
}
`)).ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
	use := analyzer.Functions()["Use"][0]
	for index, want := range []bool{false, false, false, true} {
		parameter := use.Parameters[index]
		if got := TriviallyDestructible(parameter.Type); got != want {
			t.Errorf("TriviallyDestructible(%s %s) = %v, want %v", parameter.Name, typeDisplayName(parameter.Type), got, want)
		}
	}
	if !analyzer.Types()["Handle"].CustomFree {
		t.Error("canonical Handle type does not record its custom free")
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics %q, want %q", len(got), got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("diagnostic %d = %q, want %q", index, got[index], want[index])
		}
	}
}
