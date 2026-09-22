package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Ordinary instance methods may copy a copyable receiver but cannot explicitly
// consume the complete self Place through discard, a consuming call, or return.
//
// Rules:
//   - rules/memory/ownership.md — §9 "Methods and self"
//   - rules/memory/copy_move.md — §18 "Methods and self"
//   - rules/declarations/functions.md — §22 "Instance methods"
func TestOrdinaryMethodCannotConsumeWholeSelf(t *testing.T) {
	errors := analyzeSource(t, `
type Item struct {
	value: int,
}

fn Take(-> item: Item) void {}

impl Item {
	fn DiscardWhole() void {
		discard self
	}

	fn PassWhole() void {
		Take(<-self)
	}

	fn ReturnWhole() Item {
		return <-self
	}

	fn CopyWhole() Item {
		return self
	}
}
`)

	if len(errors) != 3 {
		t.Fatalf("errors = %v, want three whole-self consumption diagnostics", errors)
	}
	for _, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, "ordinary method cannot consume complete self") {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
	}
}

// Stored fields inherit mutation authority from their root Place. Projection
// must not turn an immutable binding writable, while a mutable root retains
// ordinary field replacement authority.
//
// Rules:
//   - rules/memory/ownership.md — §8 "Field mutability and receiver authority"
//   - rules/memory/ownership.md — §18.3–18.4 immutable and mutable aggregates
func TestStructFieldMutabilityFollowsRootAuthority(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
type Inner struct {
	value: int,
}

type Outer struct {
	inner: Inner,
}

impl Outer {
	fn Update() void {
		self.inner.value = 5
	}
}

fn Check() void {
	let immutable := Outer { inner: Inner { value: 1 } }
	immutable.inner.value = 2

	let mut mutable := Outer { inner: Inner { value: 3 } }
	mutable.inner.value = 4
}
`)

	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot assign to immutable place immutable.inner.value") {
		t.Fatalf("field mutability diagnostics = %v", errors)
	}
	functions := analyzer.functions["Outer.Update"]
	if len(functions) != 1 || !functions[0].ReceiverMutable {
		t.Fatalf("Outer.Update receiver facts = %+v, want derived mutable receiver", functions)
	}
}

// A method may explicitly move one owned self member when the Place can be
// tracked independently. The move makes the receiver contract mutable without
// permitting consumption of complete self.
//
// Rules:
//   - rules/memory/ownership.md — §8 "Field mutability and receiver authority"
//   - rules/memory/copy_move.md — §18 "Methods and self"
func TestMethodMayConsumeOwnedSelfMember(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `
@noCopy
type Payload struct {
	value: int,
}

type Package struct {
	payload: Payload,
}

impl Package {
	fn ReleasePayload() void {
		let payload :<- self.payload
		discard payload
	}
}
`)

	if len(errors) != 0 {
		t.Fatalf("member consumption errors = %v", errors)
	}
	functions := analyzer.functions["Package.ReleasePayload"]
	if len(functions) != 1 || !functions[0].ReceiverMutable {
		t.Fatalf("Package.ReleasePayload receiver facts = %+v, want derived mutable receiver", functions)
	}
}

// An immutable aggregate may still be partially consumed: immutability bars
// replacement, not an explicit ownership transfer. Independently tracked
// sibling fields remain available after the move.
//
// Rules:
//   - rules/memory/ownership.md — §18.3 "Immutable aggregate roots"
//   - rules/memory/copy_move.md — §14 "Partial moves"
func TestImmutableAggregatePermitsExplicitPartialMove(t *testing.T) {
	errors := analyzeSource(t, `
@noCopy
type Payload struct {
	value: int,
}

type Package struct {
	payload: Payload,
	count: int,
}

fn Check() int {
	let package := Package {
		payload: Payload { value: 1 },
		count: 2,
	}
	let payload :<- package.payload
	let count := package.count
	discard payload
	return count
}
`)

	if len(errors) != 0 {
		t.Fatalf("immutable partial-move errors = %v", errors)
	}
}

// A consuming parameter requires a visible move for every reusable Place,
// including a copyable projected field. Fresh values remain marker-free, and
// an explicit field transfer leaves disjoint sibling fields available.
//
// Rules:
//   - rules/memory/copy_move.md — §§7 and 8.2
//   - rules/declarations/functions.md — §9 "Explicit consuming parameter"
func TestConsumingParameterRequiresExplicitMoveForProjectedCopyablePlace(t *testing.T) {
	errors := analyzeSource(t, `
type Pair struct {
	first: int,
	second: int,
}

fn Consume(-> value: int) void {}

fn Invalid() void {
	let pair := Pair { first: 1, second: 2 }
	Consume(pair.first)
}

fn Valid() int {
	let pair := Pair { first: 1, second: 2 }
	Consume(<-pair.first)
	Consume(3)
	return pair.second
}
`)

	if len(errors) != 1 {
		t.Fatalf("consuming-argument errors = %v, want one missing-marker diagnostic", errors)
	}
	if errors[0].ID != diagnostics.ImplicitMoveDisallowed ||
		!strings.Contains(errors[0].Message, "reusable source pair.first") ||
		!strings.Contains(errors[0].Help, "<-pair.first") {
		t.Fatalf("consuming-argument diagnostic = %+v", errors[0])
	}
}

// An explicit source move belongs to the outer call transaction. A later
// invalid argument prevents call entry and therefore must leave the earlier
// projected source owned and readable by the caller.
//
// Rules:
//   - rules/memory/copy_move.md — §8.3 "Fallible argument evaluation and commit"
//   - rules/declarations/functions.md — §18 "Call transfer commit"
func TestFailedLaterArgumentDoesNotCommitEarlierExplicitProjectedMove(t *testing.T) {
	errors := analyzeSource(t, `
type Pair struct {
	first: int,
	second: int,
}

fn Consume(-> value: int, label: string) void {}

fn Check() int {
	let pair := Pair { first: 1, second: 2 }
	Consume(<-pair.first, 3)
	return pair.first + pair.second
}
`)

	if len(errors) != 1 || !strings.Contains(errors[0].Message, "argument 2 to Consume must be string, got int") {
		t.Fatalf("call-transfer diagnostics = %v, want only the later argument mismatch", errors)
	}
}

// Plain closure capture is a copy request and never silently consumes a
// move-only source. Explicit `<-` capture consumes copyable and move-only outer
// bindings while installing a distinct available environment binding.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§15–17 "Capture forms"
//   - rules/memory/copy_move.md — §11 "Closure captures"
func TestLambdaCaptureCopyAndExplicitMoveOwnership(t *testing.T) {
	errors := analyzeSource(t, `
@noCopy
type Resource struct {
	value: int,
}

fn PlainCaptureDoesNotMove() int {
	let resource := Resource { value: 1 }
	let closure := capture(resource) fn() int {
		return resource.value
	}
	return resource.value
}

fn MoveCopyable() int {
	let value := 2
	let closure := capture(<-value) fn() int {
		return value
	}
	return value
}

fn MoveOnlyCaptureIsValid() int {
	let resource := Resource { value: 3 }
	let closure := capture(<-resource) fn() int {
		return resource.value
	}
	return closure()
}
`)

	if len(errors) != 2 {
		t.Fatalf("capture ownership errors = %v, want copy-capture and moved-source diagnostics", errors)
	}
	if errors[0].ID != diagnostics.ImplicitMoveDisallowed || !strings.Contains(errors[0].Message, "cannot copy-capture resource") || !strings.Contains(errors[0].Help, "capture(<-resource)") {
		t.Fatalf("plain capture diagnostic = %+v", errors[0])
	}
	if !strings.Contains(errors[1].Message, "use of moved value value") {
		t.Fatalf("move capture diagnostic = %+v", errors[1])
	}
}
