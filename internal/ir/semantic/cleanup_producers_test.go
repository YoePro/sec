package semantic

import (
	"errors"
	"strings"
	"testing"
)

const ownedLocalSource = `
module main

type Handle struct { id: int }
impl Handle { free {
    let n := id
    discard n
} }

type Pair struct { left: Handle, count: int, right: Handle }

fn Make(id: int) Handle {
    return Handle { id: id }
}
`

// P17-35 (package17 §§ 22, 34, 38–40; ownership amendment §§ 17, 20, 23):
// an immutable root-scope owned local is tracked after its initialization
// with Sema's action/scope and an imported destruction plan graph; every
// function exit runs cleanup.run-function after the protected return value is
// built; custom free is a lifecycle function the plan names.
func TestOwnedLocalsTrackAndRunFunctionCleanup(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
fn Use(flag: bool) int {
    let h := Make(1)
    let p := Pair { left: Make(2), count: 3, right: Make(4) }
    if flag {
        return h.id
    }
    return p.count
}

fn main() int {
    return Use(true)
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`"destroy:main:struct:Handle" !2 struct custom-free="main::@free:Handle()"`,
		`field#2="destroy:main:struct:Handle" field#0="destroy:main:struct:Handle"`,
		`cleanup.track-owned %2 action=#1 scope=#2 ordinal=1 plan="destroy:main:struct:Handle"`,
		`ordinal=2 plan="destroy:main:struct:Pair"`,
		`func "main::@free:Handle()"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "cleanup.run-function exit=return") != 2 {
		t.Fatalf("each return runs function cleanup once:\n%s", text)
	}
	for _, fn := range module.Functions {
		if fn.Name != "Use" {
			continue
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpReturn && (index == 0 || block.Operations[index-1].Kind != OpCleanupRunFunction) {
					t.Fatalf("return without preceding function cleanup:\n%s", text)
				}
			}
		}
	}
	// A plan naming a function that is not the type's free is rejected.
	for index := range module.DestructionPlans {
		if module.DestructionPlans[index].CustomFree {
			module.DestructionPlans[index].FreeFunction = "main::Make(int)"
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "not a custom free") {
		t.Fatalf("a wrong custom free must be rejected, got %v", err)
	}
}

// Forms beyond this slice keep explicit P17 boundaries and publish no module.
func TestOwnedLocalProducerBoundaries(t *testing.T) {
	prefix := ownedLocalSource + `
enum Failure error { Missing }

fn Maybe(flag: bool) Result[int, Failure] {
    if flag { return Ok(1) }
    return Err(Failure.Missing)
}
`
	for name, test := range map[string]struct{ body, want string }{
		"branch reassignment": {"fn F(flag: bool) int {\n    let mut h := Make(1)\n    if flag {\n        h = Make(2)\n    }\n    return h.id\n}\n", "outside its declaring scope"},
		"field move":          {"fn F() Handle {\n    let p := Pair { left: Make(1), count: 2, right: Make(3) }\n    return p.left\n}\n", "P17"},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, prefix+test.body+"\nfn main() int {\n    return 0\n}\n", 17)
			var unsupported *UnsupportedFeatureError
			if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, test.want) {
				t.Fatalf("want boundary %q, got module=%v err=%v", test.want, module != nil, err)
			}
		})
	}
}

// Predecessor packages keep rejecting custom free outright.
func TestCustomFreeStaysClosedBeforePackage17(t *testing.T) {
	_, err := analyzedModule(t, ownedLocalSource+"\nfn main() int {\n    return 0\n}\n", 16)
	if err == nil || !strings.Contains(err.Error(), "custom free lifecycle destruction") {
		t.Fatalf("package 16 must reject custom free, got %v", err)
	}
}

// P17-42 (package17 §§ 39–40, 44; destruction.md §§ 6–8): owned locals of
// nested and loop scopes are tracked; falling out of a scope, `continue` and
// `break` run cleanup.run-scope over exactly the scopes Sema says they leave,
// before the edge; a return from a nested scope runs function cleanup only.
func TestScopeExitsRunOwnedLocalCleanup(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
fn F(flag: bool) int {
    let a := Make(0)
    if flag {
        let b := Make(1)
        let x := b.id
        discard x
    }
    for i in 0..<3 {
        let c := Make(i)
        if i == 1 {
            continue
        }
        if i == 2 {
            break
        }
        let y := c.id
        discard y
    }
    while flag {
        let d := Make(5)
        if d.id == 5 {
            return d.id
        }
    }
    return a.id
}

fn main() int {
    return F(true)
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"cleanup.run-scope scopes=[#3] exit=fallthrough",
		"cleanup.run-scope scopes=[#5, #4] exit=continue",
		"cleanup.run-scope scopes=[#6, #4] exit=break",
		"cleanup.run-scope scopes=[#4] exit=fallthrough",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "cleanup.track-owned") != 4 || strings.Count(text, "cleanup.run-function exit=return") != 2 {
		t.Fatalf("four tracked locals and two function exits expected:\n%s", text)
	}
	// Each run-scope directly precedes the edge that leaves its scopes.
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpCleanupRunScope && (index+1 >= len(block.Operations) || block.Operations[index+1].Kind != OpBranch) {
					t.Fatalf("run-scope must precede its exit edge:\n%s", text)
				}
			}
		}
	}
}

// Programs without owned locals keep their predecessor IR: no scope or
// function cleanup is emitted for them at package 17.
func TestScopeCleanupOnlyWithOwnedLocals(t *testing.T) {
	source := "module main\n\nfn F(flag: bool) int {\n    let mut total := 0\n    for i in 0..<3 {\n        if i == 1 {\n            continue\n        }\n        total += i\n    }\n    if flag {\n        return total\n    }\n    return 0\n}\n\nfn main() int {\n    return F(true)\n}\n"
	predecessor, err := analyzedModule(t, source, 16)
	if err != nil {
		t.Fatal(err)
	}
	current, err := analyzedModule(t, source, 17)
	if err != nil {
		t.Fatal(err)
	}
	if Format(predecessor) != Format(current) {
		t.Fatal("package 17 changed a program without owned locals")
	}
}

// P17-35 moves (package17 §§ 38, 40, 55): moving a whole tracked owned local
// out by a return or into a new local is own.move + cleanup.cancel of the
// source registration (transferred / moved); the returned value leaves before
// function cleanup, and the new local is tracked as the new owner.
func TestOwnedLocalMovesCancelTheirRegistration(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
fn Out(flag: bool) Handle {
    let h := Make(1)
    if flag {
        let g :<- h
        return g
    }
    return Make(2)
}

fn main() int {
    return 0
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"cleanup.track-owned %2 action=#1",
		"cleanup.cancel action=#1 reason=moved",
		"cleanup.cancel action=#2 reason=transferred",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, fn := range module.Functions {
		if fn.Name != "Out" {
			continue
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpCleanupCancel && (index == 0 || block.Operations[index-1].Kind != OpOwnMove) {
					t.Fatalf("a cancel follows the move it ends:\n%s", text)
				}
			}
		}
	}
	// A cancel of an action that was never registered is rejected.
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if block.Operations[index].Kind == OpCleanupCancel {
					block.Operations[index].Cleanup.Action = 99
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "unregistered action") {
		t.Fatalf("an unregistered cancel must be rejected, got %v", err)
	}
}

// P17-23/P17-42 error propagation (package17 §§ 40, 46): a bodyless try that
// propagates — a Result call or checked arithmetic — runs function cleanup
// with exit kind error-propagation after the error is built, before its
// return; the success path keeps its ordinary return cleanup.
func TestErrorPropagationRunsFunctionCleanup(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
enum Failure error { Missing }

fn Maybe(flag: bool) Result[int, Failure] {
    if flag { return Ok(1) }
    return Err(Failure.Missing)
}

fn F() Result[int, Failure] {
    let h := Make(1)
    let v := try Maybe(true)
    return Ok(v + h.id)
}

fn G(a: int32, b: int32) Result[int32, ArithmeticError] {
    let h := Make(1)
    let s := try a + b
    let id := h.id
    discard id
    return Ok(s)
}

fn main() int {
    return 0
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if strings.Count(text, "cleanup.run-function exit=error-propagation") != 2 || strings.Count(text, "cleanup.run-function exit=return") != 2 {
		t.Fatalf("two propagation and two ordinary exits expected:\n%s", text)
	}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpReturn && fn.Name != "main" && fn.Name != "Maybe" && fn.Name != "Make" && fn.Name != "@free:Handle" &&
					(index == 0 || block.Operations[index-1].Kind != OpCleanupRunFunction) {
					t.Fatalf("return without function cleanup in %s:\n%s", fn.Name, text)
				}
			}
		}
	}
}

// Owned parameters and argument moves (functions.md § 6; destruction.md
// § 9.2; package17 § 67): the callee tracks an owned by-value parameter on
// entry and destroys it at exit unless it moves it out; the caller passes an
// owned local with `<-` (own.move, argument action move) and cancels the
// source registration only when the call commits, after every argument was
// prepared; a fresh temporary is forwarded with action move.
func TestOwnedParametersAndArgumentMoves(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
fn Consume(h: Handle) int {
    return h.id
}

fn Forward(h: Handle) Handle {
    return h
}

fn Two(h: Handle, n: int32) int {
    return h.id
}

fn Pass(a: int32, b: int32) int {
    let g := Make(1)
    let first := Two(<-g, try a + b {
        Err(_) => 0
    })
    discard first
    return Consume(Make(3))
}

fn main() int {
    return 0
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"Consume(%0 h: !2 [owned])",
		"cleanup.track-owned %0 action=#1",
		"cleanup.cancel action=#2 reason=transferred",
		`"main::Consume(main::Handle)"(%`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "[move]") != 2 {
		t.Fatalf("two move arguments expected:\n%s", text)
	}
	for _, fn := range module.Functions {
		if fn.Name != "Pass" {
			continue
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpCleanupCancel && (index+1 >= len(block.Operations) || block.Operations[index+1].Kind != OpDirectCall) {
					t.Fatalf("an argument move cancels directly before its call commits:\n%s", text)
				}
			}
		}
	}
	// A move argument whose action claims a copy is rejected.
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				op := &block.Operations[index]
				for position, action := range op.ArgumentActions {
					if action == ArgumentMove {
						op.ArgumentActions[position] = ArgumentCopyTrivial
					}
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), `want "move"`) {
		t.Fatalf("an owned argument must be passed by move, got %v", err)
	}
}

// Mutable owned storage (package17 §§ 28, 31–33, 54–55; destruction.md § 13):
// `let mut` initializes address-taken storage through its Place and tracks
// the Place; fields are read through Place projections, never by copying the
// whole value; replacement destroys the old value with its plan and moves the
// registration to a new incarnation; a move out is own.move-from-place with a
// cancel, and reinitialization registers again.
func TestMutableOwnedStorage(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
fn Straight() Handle {
    let mut h := Make(1)
    let before := h.id
    h = Make(2)
    let g :<- h
    h = Make(before)
    let x := g.id
    discard x
    return h
}

fn main() int {
    return 0
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"address-taken",
		"own.initialize-place %2, %1 cleanup=#0->#1",
		`own.replace-place %8, %7 plan="destroy:main:struct:Handle" cleanup=#1->#2`,
		"cleanup.cancel action=#1 reason=replaced",
		"own.move-from-place %9",
		"cleanup.cancel action=#2 reason=moved",
		"cleanup=#0->#4",
		"cleanup.cancel action=#4 reason=transferred",
		"place.field %3 field=#0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "storage.load") {
		t.Fatalf("owned storage is never loaded as a whole value:\n%s", text)
	}
}

// Value-producing try-handler blocks (errorhandling.md § 21.1; package17
// §§ 39–40): the recovery value is built first, then leaving the handler
// block runs cleanup.run-scope for its owned locals, then control branches to
// the try merge with the value.
func TestValueProducingHandlerBlockRunsScopeCleanup(t *testing.T) {
	module, err := analyzedModule(t, ownedLocalSource+`
enum Failure error { Missing }

fn Maybe(flag: bool) Result[int, Failure] {
    if flag { return Ok(1) }
    return Err(Failure.Missing)
}

fn F() int {
    let v := try Maybe(true) {
        Err(_) => {
            let h := Make(1)
            let z := h.id
            z + 1
        }
    }
    return v
}

fn main() int {
    return 0
}
`, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if !strings.Contains(text, "cleanup.run-scope scopes=[#5] exit=fallthrough") {
		t.Fatalf("the handler block runs its scope cleanup:\n%s", text)
	}
	for _, fn := range module.Functions {
		if fn.Name != "F" {
			continue
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpCleanupRunScope && (index+1 >= len(block.Operations) || block.Operations[index+1].Kind != OpBranch || len(block.Operations[index+1].Successors[0].Arguments) != 1) {
					t.Fatalf("run-scope precedes the branch carrying the recovery value:\n%s", text)
				}
			}
		}
	}
}
