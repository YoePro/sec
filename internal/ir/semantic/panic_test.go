package semantic

import (
	"strings"
	"testing"
)

const panicSource = `module main

fn Check(count: int) int {
    assert count > 0, "count must be positive"
    if count > 100 {
        panic "too many"
    }
    return count
}

fn Proven(count: int) int {
    if count > 0 {
        assert count > 0
    }
    return 1
}

fn Mode(code: int) int {
    if code == 1 {
        return 10
    }
    unreachable
}

@noPanic
fn Safe(flag: bool) int {
    if flag {
        return 1
    }
    return 2
}
`

func panicOperations(function *Function) []Operation {
	var result []Operation
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpPanic {
				result = append(result, op)
			}
		}
	}
	return result
}

// Explicit panic, unproven assertions, and checked unreachable lower to the
// non-returning panic operation with the registered reason, the static
// message, and the source location; a proven assertion emits no check; the
// verified @noPanic guarantee is carried on the function.
//
// Rules:
//   - rules/compiler/semantic_ir.md — §§ 55–57
//   - rules/errors/panic.md — §§ 15–16, § 21
func TestPanicAssertionAndUnreachableLowerToExplicitPanic(t *testing.T) {
	module, err := analyzedModule(t, panicSource, 14)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}
	check := panicOperations(loopFunction(t, module, "Check"))
	if len(check) != 2 || check[0].PanicReason != "AssertionFailed" || check[0].PanicMessage != "count must be positive" ||
		check[1].PanicReason != "ExplicitPanic" || check[1].PanicMessage != "too many" || check[1].Location.Line != 6 {
		t.Fatalf("Check panics = %+v", check)
	}
	asserts := 0
	for _, block := range loopFunction(t, module, "Check").Blocks {
		for _, op := range block.Operations {
			if op.AssertCheck {
				asserts++
			}
		}
	}
	if asserts != 1 {
		t.Errorf("assertion checks = %d, want 1", asserts)
	}
	if proven := loopFunction(t, module, "Proven"); len(panicOperations(proven)) != 0 || countOperations(proven, OpCondBranch) != 1 {
		t.Errorf("a proven assertion emitted a check")
	}
	if mode := panicOperations(loopFunction(t, module, "Mode")); len(mode) != 1 || mode[0].PanicReason != "UnreachableReached" || mode[0].PanicHasMessage {
		t.Errorf("Mode panics = %+v", mode)
	}
	if safe := loopFunction(t, module, "Safe"); !safe.NoPanic || safe.NoPanicSource != "written" {
		t.Errorf("Safe noPanic = %v %q", safe.NoPanic, safe.NoPanicSource)
	}
	text := Format(module)
	for _, fragment := range []string{"conditional-branch assert", `panic reason=AssertionFailed#6 message="count must be positive"`, "panic reason=UnreachableReached#7", "noPanic(written)"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("format is missing %q", fragment)
		}
	}
}

// The verifier rejects an unregistered panic reason and a @noPanic function
// that still reaches a panic.
func TestVerifierRejectsInvalidPanicFacts(t *testing.T) {
	for name, mutate := range map[string]func(*Module){
		"contradictory noPanic": func(module *Module) {
			check := loopFunction(t, module, "Check")
			check.NoPanic, check.NoPanicSource = true, "written"
		},
		"unregistered reason": func(module *Module) {
			for _, block := range loopFunction(t, module, "Mode").Blocks {
				for index := range block.Operations {
					if block.Operations[index].Kind == OpPanic {
						block.Operations[index].PanicReason = "Invented"
					}
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, panicSource, 14)
			if err != nil {
				t.Fatal(err)
			}
			mutate(module)
			if err := Verify(module); err == nil {
				t.Fatal("Verify accepted invalid panic facts")
			}
		})
	}
}

// An if without else whose body continues joins the merge block exactly once.
func TestIfWithoutElseJoinsOnce(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Clamp(value: int) int {
    let mut result := value
    if value > 10 {
        result = 10
    }
    return result
}
`, 14)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}
}
