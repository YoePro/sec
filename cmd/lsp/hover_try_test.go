package main

import (
	"strings"
	"testing"
)

// Result try hover consumes the exact carrier and propagation facts recorded
// by Sema, both at the try keyword and at the protected callable.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — § 12.1 and § 37.10
func TestTryHoverShowsResolvedResultPropagationAtKeywordAndOperand(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Failed)
}

fn Use() Result[int, error] {
    let value := try Read()
    return Ok(value)
}
`
	tryStart := strings.Index(source, "try Read()")
	for name, offset := range map[string]int{
		"keyword": tryStart + 1,
		"operand": tryStart + len("try "),
	} {
		t.Run(name, func(t *testing.T) {
			hover, ok := hoverForSource("", source, offsetPosition(source, offset))
			if !ok || !strings.Contains(hover.Contents.Value, "Protected carrier: `Result[int, ReadError]`") ||
				!strings.Contains(hover.Contents.Value, "Success value: `int`") ||
				!strings.Contains(hover.Contents.Value, "Failure handling: `propagated`") ||
				!strings.Contains(hover.Contents.Value, "Propagated error: `ReadError`") ||
				!strings.Contains(hover.Contents.Value, "Propagation target: `Result[int, error]`") ||
				!strings.Contains(hover.Contents.Value, "Err consumed by: `enclosing function return`") {
				t.Fatalf("try propagation hover = %+v, %v", hover, ok)
			}
		})
	}
}

// Local try hover presents the handler plan selected by Sema and does not
// mistake it for bodyless propagation.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — §§ 15–16 and § 37.10
func TestTryHoverDistinguishesResolvedLocalHandling(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Failed)
}

fn Use() int {
    return try Read() {
        Err(_) => 0
    }
}
`
	offset := strings.LastIndex(source, "Read()")
	hover, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "Protected carrier: `Result[int, ReadError]`") ||
		!strings.Contains(hover.Contents.Value, "Failure handling: `local try handlers`") ||
		!strings.Contains(hover.Contents.Value, "Error channel: `ReadError`") ||
		!strings.Contains(hover.Contents.Value, "Err consumed by: `local handler`") ||
		!strings.Contains(hover.Contents.Value, "Handler coverage: `exhaustive`") ||
		!strings.Contains(hover.Contents.Value, "Resolved handlers: `1`") ||
		strings.Contains(hover.Contents.Value, "Propagation target") {
		t.Fatalf("local try hover = %+v, %v", hover, ok)
	}
}

// Option try hover presents Some as the success path and None as ordinary
// propagated absence. It must not reuse Result's failure/error vocabulary.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — §§ 9, 12.2 and § 37.4
//   - rules/corrections/applied/lsp-errorhandling-correction-20260824.md — "Try hover"
func TestTryHoverShowsResolvedOptionPropagationAtKeywordAndOperand(t *testing.T) {
	source := `module main

fn Find() Option[int] {
    return None
}

fn Use() Option[int] {
    let value := try Find()
    return Some(value)
}
`
	tryStart := strings.Index(source, "try Find()")
	for name, offset := range map[string]int{
		"keyword": tryStart + 1,
		"operand": tryStart + len("try "),
	} {
		t.Run(name, func(t *testing.T) {
			hover, ok := hoverForSource("", source, offsetPosition(source, offset))
			contents := hover.Contents.Value
			if !ok || !strings.Contains(contents, "Protected carrier: `Option[int]`") ||
				!strings.Contains(contents, "Success value: `int`") ||
				!strings.Contains(contents, "Success state: `Some(int)`") ||
				!strings.Contains(contents, "Try expression type: `int`") ||
				!strings.Contains(contents, "Absence handling: `propagated`") ||
				!strings.Contains(contents, "Propagated state: `None`") ||
				!strings.Contains(contents, "Propagation target: `Option[int]`") ||
				!strings.Contains(contents, "None consumed by: `enclosing function return`") ||
				strings.Contains(contents, "Failure handling") ||
				strings.Contains(contents, "Propagated error") ||
				strings.Contains(contents, "Err consumed") {
				t.Fatalf("Option try propagation hover = %+v, %v", hover, ok)
			}
		})
	}
}

// Local Option try hover exposes Sema's exact None-handler plan, including
// exhaustive recovery and guarded partial recovery whose unmatched None still
// propagates through the enclosing Option return.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", locally handled and unhandled states
//   - rules/errors/errorhandling.md — §§ 15–16 and § 37.4
//   - rules/corrections/applied/lsp-errorhandling-correction-20260824.md — "Try hover"
func TestTryHoverExplainsLocalOptionNoneRecovery(t *testing.T) {
	source := `module main

fn Find() Option[int] {
    return None
}

fn Recover() int {
    return try Find() {
        None => 0
    }
}

fn RecoverWhen(flag: bool) Option[int] {
    let value := try Find() {
        None where flag => 0
    }
    return Some(value)
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}

	exhaustive := hoverAt("try Find()")
	exhaustiveStart := strings.Index(source, "try Find()")
	exhaustiveOperand, ok := hoverForSource("", source, offsetPosition(source, exhaustiveStart+len("try ")))
	if !ok || exhaustiveOperand.Contents.Value != exhaustive {
		t.Fatalf("protected Option operand hover differs from try keyword hover: %+v, %v", exhaustiveOperand, ok)
	}
	for _, want := range []string{
		"Protected carrier: `Option[int]`",
		"Success state: `Some(int)`",
		"Absence handling: `local None handlers`",
		"Handler coverage: `exhaustive`",
		"Resolved handlers: `1`",
		"Handler 1: `None` → recovery value",
		"None consumed by: `local handler`",
	} {
		if !strings.Contains(exhaustive, want) {
			t.Fatalf("exhaustive Option hover missing %q:\n%s", want, exhaustive)
		}
	}
	if strings.Contains(exhaustive, "Propagation target") || strings.Contains(exhaustive, "Unhandled None") {
		t.Fatalf("exhaustive Option hover claims residual propagation:\n%s", exhaustive)
	}

	// The first occurrence belongs to Recover; select the guarded occurrence.
	guardedOffset := strings.LastIndex(source, "try Find()") + 1
	guarded, ok := hoverForSource("", source, offsetPosition(source, guardedOffset))
	if !ok {
		t.Fatal("missing hover for guarded Option try")
	}
	partial := guarded.Contents.Value
	for _, want := range []string{
		"Handler coverage: `partial`",
		"Handler 1: `None where …` → recovery value",
		"Unhandled None: `propagated`",
		"Propagation target: `Option[int]`",
	} {
		if !strings.Contains(partial, want) {
			t.Fatalf("partial Option hover missing %q:\n%s", want, partial)
		}
	}
}

// Try hover lists the compiler-internal failure set with each protected
// source operation, describes every resolved handler, and states when a
// concrete error widens into the open error root.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
//   - rules/errors/errorhandling.md — §11, §2.1, §36 "LSP requirements"
func TestTryHoverShowsFailureSetHandlersAndWidening(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Failed)
}

fn Widen() Result[int, error] {
    return Ok(try Read())
}

fn Mixed(values: int[], index: uint, amount: int) int {
    return try values[index] + amount {
        Err(IndexError.OutOfBounds) => 0
        Err(_) => 1
    }
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}
	widen := hoverAt("try Read()")
	if !strings.Contains(widen, "Error identity: `ReadError` widened to `error`; the concrete error identity is retained") {
		t.Fatalf("widening hover = %s", widen)
	}
	mixed := hoverAt("try values")
	for _, want := range []string{
		"Failure source: `values[index]` (bounds) may raise `IndexError`",
		"Failure source: `(values[index] + amount)` (arithmetic) may raise `ArithmeticError`",
		"Failure set: `IndexError`, `ArithmeticError`",
		"Handler 1: `Err(IndexError.OutOfBounds)` → recovery value",
		"Handler 2: `Err(_)` → recovery value",
		"Handler coverage: `exhaustive`",
	} {
		if !strings.Contains(mixed, want) {
			t.Fatalf("failure-set hover missing %q:\n%s", want, mixed)
		}
	}
}

// Hover on the try keyword of a fallible assignment presents the resolved
// error channel and its propagation or local-handler plan.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment", §36
func TestTryAssignmentHoverShowsResolvedPlan(t *testing.T) {
	source := `module main

enum SpeedError error {
    TooHigh,
    Negative,
}

type Vehicle struct {
    speed: int,
}

impl Vehicle {
    property TopSpeed: int {
        get {
            return self.speed
        }
        try set next SpeedError {
            self.speed = next
        }
    }
}

fn Propagate(vehicle: ref mut Vehicle) Result[void, SpeedError] {
    try vehicle.TopSpeed = 10
    return Ok()
}

fn Handle(vehicle: ref mut Vehicle) Result[void, SpeedError] {
    try vehicle.TopSpeed = 20 {
        Err(SpeedError.TooHigh) => {
        }
    }
    return Ok()
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}
	propagated := hoverAt("try vehicle.TopSpeed = 10")
	for _, want := range []string{"### `try` assignment", "Error channel: `SpeedError`", "Failure handling: `propagated`", "Propagation target: `Result[void, SpeedError]`"} {
		if !strings.Contains(propagated, want) {
			t.Fatalf("propagated assignment hover missing %q:\n%s", want, propagated)
		}
	}
	handled := hoverAt("try vehicle.TopSpeed = 20")
	for _, want := range []string{"Failure handling: `local try handlers`", "Handler coverage: `partial`", "Handler 1: `Err(TooHigh)`", "Unhandled errors: `propagated to Result[void, SpeedError]`"} {
		if !strings.Contains(handled, want) {
			t.Fatalf("handled assignment hover missing %q:\n%s", want, handled)
		}
	}
}

// Function hover presents the compiler-owned panic summary: panic-free
// callables say so, and panic-capable ones list each direct source with its
// registered reason and the path to a transitive source.
//
// Rules:
//   - rules/errors/panic.md — § 21 "@noPanic", registered panic reasons
//   - rules/tooling/lsp.md — "Hover"
func TestFunctionHoverShowsPanicSummary(t *testing.T) {
	source := `module main

fn Pick(values: ref int[], index: uint) int {
    return values[index]
}

fn Outer(values: ref int[]) int {
    return Pick(values, 0)
}

fn Safe(value: int) int {
    return value
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + len("fn ")
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}
	if pick := hoverAt("fn Pick"); !strings.Contains(pick, "May panic: `yes`") || !strings.Contains(pick, "Panic source: `may-panic-bounds` at 4:18 (BoundsFailure)") {
		t.Fatalf("Pick hover = %s", pick)
	}
	if outer := hoverAt("fn Outer"); !strings.Contains(outer, "Panic path: `Outer` -> `Pick`") {
		t.Fatalf("Outer hover = %s", outer)
	}
	if safe := hoverAt("fn Safe"); !strings.Contains(safe, "May panic: `no`") {
		t.Fatalf("Safe hover = %s", safe)
	}
}

// Hover on assert presents the resolved proof state, the panic reason of an
// unproven assertion, the static message, and the provided refinement.
//
// Rules:
//   - rules/errors/panic.md — § 15.6, § 15.8, § 28
func TestAssertionHoverShowsProofAndRefinement(t *testing.T) {
	source := `module main

type Small int range 1..9

fn Use(small: Small, value: int) int {
    assert small >= 1
    assert value > 3, "value must exceed three"
    return value
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}
	proven := hoverAt("assert small")
	if !strings.Contains(proven, "Proof: `proven` — no runtime check and no panic effect") {
		t.Fatalf("proven assertion hover = %s", proven)
	}
	unproven := hoverAt("assert value")
	for _, want := range []string{"Proof: `not proven` — checked at run time; failure panics with `AssertionFailed`", "Message: `value must exceed three`", "Refinement: following code may rely on `(value > 3)`"} {
		if !strings.Contains(unproven, want) {
			t.Fatalf("unproven assertion hover missing %q:\n%s", want, unproven)
		}
	}
}

// Call hierarchy items mark callables that may panic, so a transitive
// panic-capable path is visible while navigating callers and callees.
//
// Rules:
//   - rules/errors/panic.md — § 28 "Diagnostics and tooling"
func TestCallHierarchyMarksPanicCapableCallables(t *testing.T) {
	source := `module main

fn Pick(values: ref int[], index: uint) int {
    return values[index]
}

fn Safe(value: int) int {
    return value
}

fn Outer(values: ref int[]) int {
    return Pick(values, 0) + Safe(1)
}
`
	uri := "file:///tmp/sec-lsp-call-hierarchy-panic/main.sec"
	items := callHierarchyItemsForSource(uri, source, offsetPosition(source, strings.Index(source, "Outer(")))
	if len(items) != 1 || !strings.Contains(items[0].Detail, "may panic") {
		t.Fatalf("prepare items = %+v, want Outer marked may panic", items)
	}
	details := map[string]string{}
	for _, call := range callHierarchyOutgoingCallsForSource(uri, source, items[0].Data.NodeID) {
		details[call.To.Name] = call.To.Detail
	}
	if !strings.Contains(details["Pick"], "may panic") || strings.Contains(details["Safe"], "may panic") {
		t.Fatalf("outgoing details = %v", details)
	}
}

// Try hover states whether each named handler binding copies the error
// payload or takes ownership of it.
//
// Rules:
//   - rules/errors/errorhandling.md — §20 "Handler ownership and guards"
func TestTryHoverShowsHandlerBindingOwnership(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

fn Work() int {
    return 1
}

type LaunchFailure union error {
    Abandoned(Task[int]),
}

fn Read() Result[int, ReadError] {
    return Ok(1)
}

fn Launch() Result[int, LaunchFailure] {
    return Ok(1)
}

fn Copies() int {
    return try Read() {
        Err(failure) => {
            discard failure
            0
        }
    }
}

fn Moves() Result[int, LaunchFailure] {
    let value := try Launch() {
        Err(failure) => return Err(failure)
    }
    return Ok(value)
}
`
	hoverAt := func(needle string) string {
		t.Helper()
		offset := strings.Index(source, needle) + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok {
			t.Fatalf("missing hover at %q", needle)
		}
		return hover.Contents.Value
	}
	if copies := hoverAt("try Read()"); !strings.Contains(copies, "`Err(failure: ReadError) (copies payload)`") {
		t.Fatalf("copy hover = %s", copies)
	}
	if moves := hoverAt("try Launch()"); !strings.Contains(moves, "`Err(failure: LaunchFailure) (moves payload)`") {
		t.Fatalf("move hover = %s", moves)
	}
}
