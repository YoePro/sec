package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

const atomicCapabilitySource = `module main

type Count uint64

static let Hits: Atomic[Count] := Atomic(Count(0))
static let Total: Atomic[uint64] := Atomic(uint64(0))

fn Sync() void {
    atomic.Fence(MemoryOrder.SeqCst)
}

@noBlock
fn Fast() uint64 {
    return Total.Load()
}

fn Orphan() uint64 {
    return Total.Load()
}

fn main() int {
    discard Hits.Load()
    discard Total.Swap(uint64(1))
    discard Fast()
    Sync()
    return 0
}
`

func analyzeAtomicCapabilities(t *testing.T, plan layout.ResolvedScalarPlan) (*Analyzer, []Error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(atomicCapabilitySource, "atomic.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(plan)
	return a, a.Analyze(program)
}

func atomicCapabilityPlan(t *testing.T) layout.ResolvedScalarPlan {
	t.Helper()
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic verified atomic minima (layout.md § 47.6); not a registered target fact.
	plan.AtomicAlignments = layout.ScalarAlignments{{Class: layout.IntegerStorage, WidthBits: 64}: 8}
	return plan
}

func atomicResults(a *Analyzer, line int) []ResolvedAtomicCapability {
	var out []ResolvedAtomicCapability
	for _, result := range a.AtomicCapabilities() {
		if result.Source.Line == line {
			out = append(out, result)
		}
	}
	return out
}

// TestAtomicCapabilityContexts derives the invocation contracts from the call
// graph: main's callees run Ordinary (T22); @noBlock adds NoBlocking (the
// interrupt attributes that add InterruptSafe and the Interrupt category are
// Sec 0.5 and not yet accepted by Sema);
// a callable no root reaches keeps an unresolved-reachability obligation
// instead of an assumed Ordinary context.
// Rules: rules/concurrency/atomics.md — §§ 37A(8), 39(6); rules/platform/target_profiles.md — § 19.
func TestAtomicCapabilityContexts(t *testing.T) {
	a, errors := analyzeAtomicCapabilities(t, atomicCapabilityPlan(t))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	fence := atomicResults(a, 9)
	if len(fence) != 1 || fence[0].Requirement.Key.Context != layout.AtomicContextOrdinary || fence[0].Requirement.Key.NoBlocking || fence[0].Requirement.Key.InterruptSafe {
		t.Fatalf("Sync fence contexts: %+v", fence)
	}
	if fence[0].Requirement.Key.Operation != "Fence" || fence[0].Requirement.Key.CarrierTypeID != "" || fence[0].Requirement.Key.SizeBytes != 0 {
		t.Fatalf("a fence has no carrier or layout (T20): %+v", fence[0].Requirement)
	}
	fast := atomicResults(a, 14)
	if len(fast) != 1 || !fast[0].Requirement.Key.NoBlocking || fast[0].Requirement.Key.InterruptSafe {
		t.Fatalf("@noBlock contract: %+v", fast)
	}
	orphan := atomicResults(a, 18)
	if len(orphan) != 1 || orphan[0].Availability != layout.AtomicUnknown || orphan[0].Missing != "unresolved execution-context reachability" {
		t.Fatalf("unreached callable must stay an open obligation: %+v", orphan)
	}
}

// TestAtomicCapabilityQuery queries the canonical plan service by carrier key
// after source validation: a named Count and uint64 share the uint64 carrier
// and the Atomic[T] layout's access facts (T02); a plan without a fact gives
// Unknown and analysis continues (T50, T51); a supported fact is retained as
// evidence (T48); a proven Unsupported fact is the focused S1261 error naming
// target, operation, order and context (T49); and a registered target without
// atomic storage facts keeps an incomplete-layout obligation (T44).
// Rules: rules/concurrency/atomics.md — §§ 37A(4)-(7), 37A(9)-(13), 48(4).
func TestAtomicCapabilityQuery(t *testing.T) {
	plan := atomicCapabilityPlan(t)
	a, errors := analyzeAtomicCapabilities(t, plan)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	hits, swap := atomicResults(a, 22), atomicResults(a, 23)
	if len(hits) != 1 || len(swap) != 1 {
		t.Fatalf("main operations: %+v %+v", hits, swap)
	}
	carrier := AtomicValueTypeIdentity(builtinType("uint64")).CarrierTypeID
	want := layout.AtomicCapabilityKey{CarrierTypeID: carrier, Representation: layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 64},
		SizeBytes: 8, AlignmentBytes: 8, Operation: "Load", Order: "SeqCst", Context: layout.AtomicContextOrdinary}
	if hits[0].Requirement.Key != want || !strings.Contains(hits[0].Requirement.SemanticTypeID, "Count") || hits[0].Availability != layout.AtomicUnknown ||
		!strings.Contains(hits[0].Missing, "no capability fact") {
		t.Fatalf("Atomic[Count].Load requirement: %+v", hits[0])
	}

	supported := layout.AtomicCapability{Key: hits[0].Requirement.Key, Availability: layout.AtomicSupported, Provenance: "test-only",
		Implementation: layout.AtomicImplementation{Kind: layout.AtomicNativeInstruction, Identity: "test-load64", Blocking: layout.AtomicPropertyNo}}
	denied := layout.AtomicCapability{Key: swap[0].Requirement.Key, Availability: layout.AtomicUnsupported, Provenance: "test-only", Reason: "test target has no 64-bit swap."}
	withFacts, err := plan.WithAtomicCapabilities([]layout.AtomicCapability{supported, denied})
	if err != nil {
		t.Fatal(err)
	}
	a, errors = analyzeAtomicCapabilities(t, withFacts)
	if len(errors) != 1 || errors[0].ID != diagnostics.AtomicTargetCapability || !strings.Contains(errors[0].Message, "Swap with order SeqCst in Ordinary context") || !strings.Contains(errors[0].Help, "64-bit swap") {
		t.Fatalf("Unsupported must be the focused S1261 error: %v", errors)
	}
	if got := atomicResults(a, 22); len(got) != 1 || got[0].Availability != layout.AtomicSupported || got[0].Capability.Implementation.Identity != "test-load64" {
		t.Fatalf("supported evidence: %+v", got)
	}
	// Total.Load in Fast shares the carrier key but needs NoBlocking: no entry.
	if got := atomicResults(a, 14); len(got) != 1 || got[0].Availability != layout.AtomicUnknown {
		t.Fatalf("a NoBlocking request is not covered by the Ordinary fact: %+v", got)
	}

	registered, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	bare, _ := registered.ScalarPlan()
	a, errors = analyzeAtomicCapabilities(t, bare)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if got := atomicResults(a, 22); len(got) != 1 || got[0].Availability != layout.AtomicUnknown || !strings.Contains(got[0].Missing, "incomplete atomic layout") {
		t.Fatalf("registered target without atomic facts: %+v", got)
	}
}

// TestAtomicCapabilityTaskContext: an operation reached only through a
// spawned logical task runs in the Task category, never Thread merely
// because tasks execute on physical workers (T24); a helper reached from both
// main and the task keeps both contracts.
// Rules: rules/concurrency/atomics.md — § 37A(8).
func TestAtomicCapabilityTaskContext(t *testing.T) {
	source := `module main

static let Total: Atomic[uint64] := Atomic(uint64(0))

fn Shared() uint64 {
    return Total.Load()
}

fn Worker() void {
    discard Total.Swap(uint64(2))
    discard Shared()
}

fn main() void {
    discard Shared()
    let worker := try spawn Worker() { Err(_) => { return } }
    detach worker
}
`
	p := parser.New(lexer.NewWithFile(source, "task.sec"))
	program := p.ParseProgram()
	a := NewAnalyzerWithScalarPlan(atomicCapabilityPlan(t))
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	worker := atomicResults(a, 10)
	if len(worker) != 1 || worker[0].Requirement.Key.Context != layout.AtomicContextTask {
		t.Fatalf("task-only operation: %+v", worker)
	}
	shared := atomicResults(a, 6)
	if len(shared) != 2 || shared[0].Requirement.Key.Context != layout.AtomicContextOrdinary || shared[1].Requirement.Key.Context != layout.AtomicContextTask {
		t.Fatalf("shared helper keeps both contracts: %+v", shared)
	}
}
