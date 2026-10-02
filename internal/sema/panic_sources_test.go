package sema

import (
	"strings"
	"testing"
)

// Dynamic-array and slice indexing and run-time conversion into a constrained
// named type are panic-capable outside try: @noPanic rejects them, while a
// try or fallible assignment that converts the check removes the panic.
// Constant conversions are proven at compile time and never panic.
//
// Rules:
//   - rules/errors/panic.md — BoundsFailure, ContractFailure, § 21 "@noPanic"
//   - rules/errors/runtime_checks.md — "Index checks", "Type contracts"
//   - rules/types/contracts.md — runtime conversion is fallible
func TestSequenceIndexAndContractConversionArePanicSources(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Percent int range 0..100

fn Element(values: int[], index: uint) int {
    return values[index]
}

fn SliceElement(values: ref int[], index: uint) int {
    return values[index]
}

fn Convert(raw: int) Percent {
    return Percent(raw)
}

fn Constant() Percent {
    return Percent(50)
}

fn TryElement(values: int[], index: uint) Result[int, IndexError] {
    return Ok(try values[index])
}

fn TryConvert(raw: int) Result[Percent, ContractError] {
    return Ok(try Percent(raw))
}

fn AssignConvert(raw: int) Result[void, ContractError] {
    let mut percent: Percent := 0
    try percent = Percent(raw)
    discard percent
    return Ok()
}
`)
	assertSemaErrors(t, errors, nil)
	graph := analyzer.CallGraph()
	for name, want := range map[string]EffectKind{
		"Element":      EffectMayPanicBounds,
		"SliceElement": EffectMayPanicBounds,
		"Convert":      EffectMayPanicContract,
	} {
		summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, name))
		if !summary.MayPanic || len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != want {
			t.Fatalf("%s effects = %+v, want one %s", name, summary, want)
		}
	}
	for _, name := range []string{"Constant", "TryElement", "TryConvert", "AssignConvert"} {
		if summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, name)); summary.MayPanic {
			t.Fatalf("%s effects = %+v, want panic-free", name, summary)
		}
	}
}

// @noPanic reports the new panic sources with their cause.
func TestNoPanicRejectsSequenceIndexAndContractConversion(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Percent int range 0..100

@noPanic
fn Element(values: int[], index: uint) int {
    return values[index]
}

@noPanic
fn Convert(raw: int) Percent {
    return Percent(raw)
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want two @noPanic violations", errors)
	}
}

// @noPanic violations carry S1092, point at the panic source as related
// location, and explain the cause and fix; a transitive violation names the
// called function that introduces the panic.
//
// Rules:
//   - rules/errors/panic.md — § 21 "@noPanic"
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor"
func TestNoPanicViolationIsAMentorDiagnostic(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Pick(values: ref int[], index: uint) int {
    return values[index]
}

@noPanic
fn Direct(left: int, right: int) int {
    return left + right
}

@noPanic
fn Transitive(values: ref int[]) int {
    return Pick(values, 0)
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want two @noPanic violations", errors)
	}
	direct, transitive := errors[0], errors[1]
	if direct.ID != "S1092" || direct.PreviousLine != 10 || !strings.Contains(direct.Help, "Checked arithmetic can overflow") {
		t.Fatalf("direct violation = %+v", direct)
	}
	if transitive.ID != "S1092" || !strings.HasPrefix(transitive.Help, "The panic comes from Pick, which Transitive calls. Indexing panics") {
		t.Fatalf("transitive violation = %+v", transitive)
	}
}

// A call to an extern function is a possible foreign abort unless the extern
// declaration carries @noPanic as a trusted foreign contract; the trusted
// fact is recorded on the function, and unknown foreign behavior is never
// positive @noPanic proof.
//
// Rules:
//   - rules/errors/panic.md — § 19(2)–(3) "Foreign code and unsafe boundaries"
//   - rules/platform/ffi.md — §42 "Foreign effects"
func TestForeignCallsNeedTrustedNoPanicContract(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

@noPanic
extern "C" fn trusted_length(value: int32) int32

extern "C" fn unknown_length(value: int32) int32

@noPanic
fn UsesTrusted(value: int32) int32 {
    let mut result: int32 := 0
    unsafe {
        result = trusted_length(value)
    }
    return result
}

@noPanic
fn UsesUnknown(value: int32) int32 {
    let mut result: int32 := 0
    unsafe {
        result = unknown_length(value)
    }
    return result
}
`)
	if len(errors) != 1 || errors[0].ID != "S1092" || !strings.Contains(errors[0].Message, "may-panic-foreign") ||
		!strings.Contains(errors[0].Help, "mark the extern declaration @noPanic as a trusted foreign contract") {
		t.Fatalf("errors = %+v, want one foreign-abort @noPanic violation", errors)
	}
	trusted := analyzer.Functions()["trusted_length"]
	unknown := analyzer.Functions()["unknown_length"]
	if len(trusted) != 1 || !trusted[0].TrustedNoPanic || len(unknown) != 1 || unknown[0].TrustedNoPanic {
		t.Fatalf("trusted foreign contract facts = %+v / %+v", trusted, unknown)
	}
}

// A call through a function value whose targets are unknown cannot be proven
// panic-free, while a function value with a known target set is checked
// through that target.
//
// Rules:
//   - rules/errors/panic.md — § 21(3)–(4) "@noPanic"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets"
func TestNoPanicRejectsUnknownFunctionValueTargets(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Safe(value: int) int {
    return value
}

@noPanic
fn CallsParameter(callback: fn(int) int) int {
    return callback(1)
}

@noPanic
fn CallsKnownSafe() int {
    let callback := Safe
    return callback(1)
}
`)
	if len(errors) != 1 || errors[0].ID != "S1092" || !strings.Contains(errors[0].Message, "function CallsParameter does not satisfy @noPanic: reachable may-panic-unknown-callee") ||
		!strings.Contains(errors[0].Help, "target is not known here") {
		t.Fatalf("errors = %+v", errors)
	}
}

// Methods called through an interface reference or a constrained generic
// parameter have no concrete body at the call, so @noPanic cannot rely on
// them; calls inside defer still count toward the function.
//
// Rules:
//   - rules/errors/panic.md — § 21(3)–(4) "@noPanic", § 11.1 "Defer"
func TestNoPanicRejectsInterfaceAndGenericDispatch(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

interface Reader {
    fn Read() int
}

@noPanic
fn ViaInterface(reader: ref Reader) int {
    return reader.Read()
}

@noPanic
fn ViaGeneric[T: Reader](reader: ref T) int {
    return reader.Read()
}

@noPanic
fn WithDefer(value: int) int {
    defer {
        let sum := value + 1
        discard sum
    }
    return value
}
`)
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want interface, generic, and defer violations", errors)
	}
	for index, want := range []string{"may-panic-unknown-callee", "may-panic-unknown-callee", "may-panic-arithmetic"} {
		if errors[index].ID != "S1092" || !strings.Contains(errors[index].Message, want) {
			t.Fatalf("error %d = %+v, want %s", index, errors[index], want)
		}
	}
}
