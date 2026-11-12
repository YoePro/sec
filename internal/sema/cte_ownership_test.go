package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Semantic CTE executes reference parameters and defer: a `ref mut`
// argument's Place observes the callee's mutations, deferred blocks run at
// invocation exit in reverse registration order, and a defer registered in
// a loop reads the binding instance of its own iteration (1..4 is
// inclusive, so the loop registers four defers that run as 4, 3, 2, 1).
// Rules: rules/compiler/compile_time_evaluation.md — §§ 20, 28;
// rules/control-flow/defer.md — §§ 3, 5, 6, 9, 13.
func TestSemanticCTEReferenceParametersAndDefer(t *testing.T) {
	source := `module main

type Acc struct {
    v: int,
}

fn Bump(acc: ref mut Acc) void {
    defer {
        acc.v = acc.v * 10
    }
    acc.v = acc.v + 1
}

fn Order(acc: ref mut Acc) void {
    for i in 1..4 {
        defer {
            acc.v = acc.v * 10 + i
        }
    }
}

fn Read(acc: ref Acc) int {
    return acc.v
}

fn Run() int {
    let mut first := Acc { v: 1 }
    Bump(ref mut first)
    let mut second := Acc { v: 0 }
    Order(ref mut second)
    return first.v * 1000 + Read(ref second)
}

type Result24321 int range 0..Run()

fn Main() int {
    let fits: Result24321 := 24321
    let over: Result24321 := 24322
    return 0
}
`
	errors := analyzeSourceRaw(t, source)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "24322") || !strings.Contains(errors[0].Message, "0..24321") {
		t.Fatalf("errors = %v", errors)
	}
}

// Owned values are destroyed during CTE exactly as at run time: a custom
// free runs when a binding's scope ends, a moved value is destroyed once by
// its new owner, replacement destroys the old value, discard destroys now,
// and a temporary read through a field ends with its expression. A free
// that fails makes the evaluation fail inside `free of T`.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 26, 27;
// rules/memory/destruction.md — §§ 5, 6, 9, 12, 13, 15.
func TestSemanticCTEDestruction(t *testing.T) {
	source := `module main

type Guard struct {
    armed: bool,
}

impl Guard {
    free {
        assert !self.armed, "guard still armed"
    }
}

fn Pass(g: Guard) Guard {
    return g
}

fn Disarmed() int {
    let mut g := Guard { armed: true }
    g.armed = false
    let moved := Pass(<-g)
    return 1
}

fn Leaked() int {
    let g := Guard { armed: true }
    return 2
}

fn Replaced() int {
    let mut g := Guard { armed: true }
    g = Guard { armed: false }
    return 3
}

fn Discarded() int {
    let mut g := Guard { armed: false }
    discard g
    return 4
}

fn Temporary() int {
    let armed := Guard { armed: true }.armed
    if armed {
        return 5
    }
    return 6
}

type A int range 0..Disarmed()
type B int range 0..Leaked()
type C int range 0..Replaced()
type D int range 0..Discarded()
type E int range 0..Temporary()
`
	errors := analyzeSourceRaw(t, source)
	want := []string{
		"range upper bound Leaked() failed during compile-time evaluation: assertion (!self.armed) failed: guard still armed (in Leaked -> free of Guard)",
		"range upper bound Replaced() failed during compile-time evaluation: assertion (!self.armed) failed: guard still armed (in Replaced -> free of Guard)",
		"range upper bound Temporary() failed during compile-time evaluation: assertion (!self.armed) failed: guard still armed (in Temporary -> free of Guard)",
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, text := range want {
		if errors[index].ID != diagnostics.CompileTimeEvaluationFailed || !strings.Contains(errors[index].Message, text) {
			t.Errorf("error %d = %s %q, want %q", index, errors[index].ID, errors[index].Message, text)
		}
	}
}

// Deferred blocks and automatic destruction share one LIFO order by
// registration: `first`, defer, `second` exit as second, defer, first; a
// match on a temporary destroys it once when the match ends.
// Rules: rules/memory/destruction.md — § 6.2; rules/compiler/compile_time_evaluation.md — §§ 26(5), 56.
func TestSemanticCTEUnifiedCleanupOrder(t *testing.T) {
	source := `module main

type Tag struct {
    id: int,
}

impl Tag {
    free {
        assert self.id != 3, "second destroyed"
        assert self.id != 1, "first destroyed"
    }
}

fn SecondFirst() int {
    let first := Tag { id: 0 }
    defer {
        assert false, "defer ran"
    }
    let second := Tag { id: 3 }
    return 1
}

fn DeferBeforeFirst() int {
    let first := Tag { id: 1 }
    defer {
        assert false, "defer ran"
    }
    let second := Tag { id: 0 }
    return 2
}

fn Make() Tag {
    return Tag { id: 0 }
}

fn MatchTemporary() int {
    match Make() {
        _ => return 3
    }
    return 4
}

type A int range 0..SecondFirst()
type B int range 0..DeferBeforeFirst()
type C int range 0..MatchTemporary()
`
	errors := analyzeSourceRaw(t, source)
	want := []string{"second destroyed", "defer ran"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, text := range want {
		if !strings.Contains(errors[index].Message, text) {
			t.Errorf("error %d = %q, want %q", index, errors[index].Message, text)
		}
	}
}

// Temporaries in match subjects and method receivers end with their full
// expression; a pattern binding that receives an owned
// payload destroys it when its arm ends unless it moves it on; a local
// reference binding reads and writes the Place it refers to.
// Rules: rules/memory/destruction.md — §§ 8, 9; rules/compiler/compile_time_evaluation.md — §§ 26–28.
func TestSemanticCTETemporariesAndLocalReferences(t *testing.T) {
	source := `module main

type Tag struct {
    armed: bool,
}

impl Tag {
    free {
        assert !self.armed, "armed tag destroyed"
    }

    fn Armed() bool {
        return self.armed
    }
}

type Acc struct {
    v: int,
}

fn Make(armed: bool) Tag {
    return Tag { armed: armed }
}

fn Wrap(armed: bool) Option[Tag] {
    return Some(Tag { armed: armed })
}

fn Matched() int {
    match Wrap(false) {
        Some(tag) => return 1
        None => return 2
    }
}

fn MatchedArmed() int {
    match Wrap(true) {
        Some(tag) => return 3
        None => return 4
    }
}

fn Receiver() int {
    if Make(true).Armed() {
        return 5
    }
    return 6
}

fn Kept() Tag {
    match Wrap(true) {
        Some(tag) => return tag
        None => return Tag { armed: false }
    }
}

fn KeepAndDisarm() int {
    let mut kept := Kept()
    kept.armed = false
    return 9
}

fn Through() int {
    let mut acc := Acc { v: 1 }
    let r := ref mut acc
    r.v = r.v + 41
    return acc.v
}

let A: int := Matched()
let B: int := MatchedArmed()
let C: int := Receiver()
let E: int := KeepAndDisarm()
let F: int := Through()
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "static initializer for B") || !strings.Contains(errors[0].Message, "armed tag destroyed (in MatchedArmed -> free of Tag)") ||
		!strings.Contains(errors[1].Message, "static initializer for C") || !strings.Contains(errors[1].Message, "armed tag destroyed") {
		t.Fatalf("errors = %v", errors)
	}
	want := map[string]string{"A": "1", "E": "9", "F": "42"}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		if expected, ok := want[static.Name]; ok {
			value, established := analyzer.CompileTimeValueOf(static.Declaration.Value)
			if !established || renderCompileTimeValue(value.Value) != expected {
				t.Errorf("%s = %v (%v), want %s", static.Name, renderCompileTimeValue(value.Value), established, expected)
			}
			delete(want, static.Name)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing %v", want)
	}
}

// The ownership accounting fails closed: a created custom-free value that
// was neither destroyed nor retained by the result is reported, so a free is
// never skipped silently in a position the executor does not model.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 26(5), 56.
func TestSemanticCTEOwnershipAccountingFailsClosed(t *testing.T) {
	executor := newCTEExecutor(NewAnalyzer(), nil)
	typ := Type{Kind: StructType, Name: "Tag", Named: true, CustomFree: true}
	leaked := DefaultConstant{Kind: StructType, Aggregate: &CompileTimeAggregate{Type: typ}}
	executor.assignOwner(leaked)
	if _, found := executor.unaccountedOwnership(DefaultConstant{}); !found {
		t.Fatal("an unaccounted custom-free value was not reported")
	}
	if _, found := executor.unaccountedOwnership(leaked); found {
		t.Error("a value retained by the result is accounted for")
	}
	executor.ownershipState().ended[leaked.Aggregate.Owner] = true
	delete(executor.ownershipState().live, leaked.Aggregate.Owner)
	if _, found := executor.unaccountedOwnership(DefaultConstant{}); found {
		t.Error("a destroyed value is accounted for")
	}
}

// Evaluator-local references are deterministic semantic identities: a
// reference binding, and a copy of it, denote the referenced Place (binding
// plus field path), never a host address, so writes through either reach
// the Place. A reference that would leave its binding (a function or getter
// returning a reference) is not given a copied value: the executor fails
// closed. An established value never carries the evaluator's ownership
// identities into materialization.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 28, 29, 56.
func TestSemanticCTEReferenceIdentityNeverEscapes(t *testing.T) {
	source := `module main

type Acc struct {
    v: int,
}

type Tag struct {
    id: int,
}

impl Tag {
    free {
    }
}

fn Through() int {
    let mut acc := Acc { v: 1 }
    let r := ref mut acc
    let s :<- r
    s.v = 7
    return acc.v
}

fn Pick(a: ref mut Acc) ref mut Acc {
    return a
}

fn Returned() int {
    let mut acc := Acc { v: 1 }
    let r := Pick(ref mut acc)
    r.v = 5
    return acc.v
}

fn MakeTag() Tag {
    return Tag { id: 4 }
}

let Copied: int := Through()
let Escaped: int := Returned()
let Held: Tag := MakeTag()
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 1 || errors[0].ID != diagnostics.SemanticCompileTimeExecutionUnavailable ||
		!strings.Contains(errors[0].Message, "Pick returns a reference, which the compile-time evaluator does not execute yet") {
		t.Fatalf("errors = %v", errors)
	}
	var owners func(DefaultConstant) int
	owners = func(value DefaultConstant) int {
		if value.Aggregate == nil {
			return 0
		}
		count := 0
		if value.Aggregate.Owner != 0 {
			count++
		}
		for _, field := range value.Aggregate.Fields {
			count += owners(field)
		}
		return count
	}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		value, established := analyzer.CompileTimeValueOf(static.Declaration.Value)
		switch static.Name {
		case "Copied":
			if !established || renderCompileTimeValue(value.Value) != "7" {
				t.Errorf("Copied = %s (%v)", renderCompileTimeValue(value.Value), established)
			}
		case "Held":
			if !established || renderCompileTimeValue(value.Value) != "Tag { id: 4 }" || owners(value.Value) != 0 {
				t.Errorf("Held = %s (%v), %d ownership identities escaped", renderCompileTimeValue(value.Value), established, owners(value.Value))
			}
		}
	}
}
