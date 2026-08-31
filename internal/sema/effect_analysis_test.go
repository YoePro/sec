package sema

import (
	"strings"
	"testing"
)

func TestArithmeticTryTransformsOnlyItsResolvedFailureEffect(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn PanickingAdd(left: int, right: int) int {
    return left + right
}

fn SafeAdd(left: int, right: int) Result[int, ArithmeticError] {
    return Ok(try left + right)
}

fn OperandStillPanics(left: int, right: int) Result[int, ArithmeticError] {
    return Ok(try PanickingAdd(left, right) + right)
}

fn Wrap(value: int) Result[int, ArithmeticError] {
    return Ok(value)
}

fn OrdinaryTryDoesNotHandleArgument(left: int, right: int) Result[int, ArithmeticError] {
    return Ok(try Wrap(left + right))
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	panickingID := callGraphNodeIDByName(t, graph, "PanickingAdd")
	panicking := graph.EffectSummary(panickingID)
	if !panicking.MayPanic || len(panicking.DirectEffects) != 1 || panicking.DirectEffects[0].Kind != EffectMayPanicArithmetic {
		t.Fatalf("PanickingAdd effects = %+v", panicking)
	}

	safeID := callGraphNodeIDByName(t, graph, "SafeAdd")
	if safe := graph.EffectSummary(safeID); safe.MayPanic || len(safe.DirectEffects) != 0 {
		t.Fatalf("SafeAdd effects = %+v, want panic-free arithmetic try", safe)
	}

	operandID := callGraphNodeIDByName(t, graph, "OperandStillPanics")
	operand := graph.EffectSummary(operandID)
	if !operand.MayPanic || len(operand.DirectEffects) != 0 || len(operand.PanicPath) != 2 || operand.PanicPath[1] != panickingID {
		t.Fatalf("OperandStillPanics effects = %+v, want transitive operand-call panic", operand)
	}

	ordinaryID := callGraphNodeIDByName(t, graph, "OrdinaryTryDoesNotHandleArgument")
	ordinary := graph.EffectSummary(ordinaryID)
	if !ordinary.MayPanic || len(ordinary.DirectEffects) != 1 {
		t.Fatalf("OrdinaryTryDoesNotHandleArgument effects = %+v, want argument arithmetic panic", ordinary)
	}
}

func TestInvalidArithmeticTryDoesNotBecomePositiveNoPanicEvidence(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn InvalidTry(left: int, right: int) int {
    return try left + right
}
`)
	if len(errors) != 1 {
		t.Fatalf("errors = %+v, want invalid arithmetic-try return error", errors)
	}

	graph := analyzer.CallGraph()
	id := callGraphNodeIDByName(t, graph, "InvalidTry")
	if summary := graph.EffectSummary(id); !summary.MayPanic || len(summary.DirectEffects) != 1 {
		t.Fatalf("invalid try effects = %+v, want retained arithmetic panic", summary)
	}
}

func TestFixedArrayBoundsEffectsDistinguishRuntimeAndProvenIndexes(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Slot uint range 0..<4

fn Runtime(values: int[4], index: int128) int {
    return values[index]
}

fn ProvenConstant(values: int[4]) int {
    return values[3]
}

fn ProvenRange(values: int[4], index: Slot) int {
    return values[index]
}

fn CallsRuntime(values: int[4], index: int128) int {
    return Runtime(values, index)
}

fn UnsafeRuntime(values: int[4], index: int128) int {
    unsafe {
        return values[index]
    }
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	runtimeID := callGraphNodeIDByName(t, graph, "Runtime")
	runtime := graph.EffectSummary(runtimeID)
	if !runtime.MayPanic || len(runtime.DirectEffects) != 1 || runtime.DirectEffects[0].Kind != EffectMayPanicBounds {
		t.Fatalf("Runtime effects = %+v, want one direct bounds-panic effect", runtime)
	}
	for _, name := range []string{"ProvenConstant", "ProvenRange"} {
		id := callGraphNodeIDByName(t, graph, name)
		if summary := graph.EffectSummary(id); summary.MayPanic || len(summary.DirectEffects) != 0 {
			t.Fatalf("%s effects = %+v, want proven-safe index", name, summary)
		}
	}
	callerID := callGraphNodeIDByName(t, graph, "CallsRuntime")
	caller := graph.EffectSummary(callerID)
	if !caller.MayPanic || len(caller.DirectEffects) != 0 || len(caller.PanicPath) != 2 || caller.PanicPath[1] != runtimeID {
		t.Fatalf("CallsRuntime effects = %+v, want transitive bounds-panic path", caller)
	}
	unsafeRuntime := graph.EffectSummary(callGraphNodeIDByName(t, graph, "UnsafeRuntime"))
	if !unsafeRuntime.MayPanic || len(unsafeRuntime.DirectEffects) != 1 || unsafeRuntime.DirectEffects[0].Kind != EffectMayPanicBounds {
		t.Fatalf("UnsafeRuntime effects = %+v, want unsafe block to retain ordinary bounds checking", unsafeRuntime)
	}
}

func TestFallibleFixedArrayBoundsRetainOnlyOperandEffects(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn PanickingIndex(left: int, right: int) int {
    return left / right
}

fn PanickingArray(left: int, right: int) int[4] {
    return [left / right, 1, 2, 3]
}

fn PanickingFallback(left: int, right: int) int {
    return left / right
}

fn FallibleSafe(values: int[4], index: int) Result[int, IndexError] {
    return Ok(try values[index])
}

fn FallibleIndexOperand(values: int[4], left: int, right: int) Result[int, IndexError] {
    return Ok(try values[PanickingIndex(left, right)])
}

fn FallibleArrayOperand(index: int, left: int, right: int) Result[int, IndexError] {
    return Ok(try PanickingArray(left, right)[index])
}

fn FallibleHandler(values: int[4], index: int, left: int, right: int) int {
    return try values[index] {
        Err(IndexError.OutOfBounds) => PanickingFallback(left, right)
    }
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	if summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, "FallibleSafe")); summary.MayPanic || len(summary.DirectEffects) != 0 {
		t.Fatalf("FallibleSafe effects = %+v, want panic-free fallible bounds check", summary)
	}
	for _, test := range []struct{ function, callee string }{
		{"FallibleIndexOperand", "PanickingIndex"},
		{"FallibleArrayOperand", "PanickingArray"},
		{"FallibleHandler", "PanickingFallback"},
	} {
		calleeID := callGraphNodeIDByName(t, graph, test.callee)
		summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, test.function))
		if !summary.MayPanic || len(summary.DirectEffects) != 0 || len(summary.PanicPath) != 2 || summary.PanicPath[1] != calleeID {
			t.Fatalf("%s effects = %+v, want only transitive panic from %s", test.function, summary, test.callee)
		}
	}
}

func TestInvalidBoundsTryDoesNotBecomePositiveNoPanicEvidence(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn InvalidTry(values: int[4], index: int) int {
    return try values[index]
}
`)
	if len(errors) != 1 {
		t.Fatalf("errors = %+v, want invalid bounds-try return error", errors)
	}

	graph := analyzer.CallGraph()
	id := callGraphNodeIDByName(t, graph, "InvalidTry")
	summary := graph.EffectSummary(id)
	if !summary.MayPanic || len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != EffectMayPanicBounds {
		t.Fatalf("invalid bounds try effects = %+v, want retained bounds panic", summary)
	}
}

func TestNoPanicFixedArrayIndexGuarantees(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Index4 int range 0..3

@noPanic
fn Proven(values: int[4], index: Index4) int {
    return values[index]
}

@noPanic
fn Fallible(values: int[4], index: int) Result[int, IndexError] {
    return Ok(try values[index])
}

@noPanic
fn Dynamic(values: int[4], index: int) int {
    return values[index]
}

@noPanic
unsafe fn UnsafeDynamic(values: int[4], index: int) int {
    return values[index]
}

fn PanickingIndex(left: int, right: int) int {
    return left / right
}

@noPanic
fn FallibleOperand(values: int[4], left: int, right: int) Result[int, IndexError] {
    return Ok(try values[PanickingIndex(left, right)])
}
`)
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want three @noPanic violations", errors)
	}
	wants := []struct {
		name string
		kind string
	}{
		{"Dynamic", string(EffectMayPanicBounds)},
		{"UnsafeDynamic", string(EffectMayPanicBounds)},
		{"FallibleOperand", string(EffectMayPanicArithmetic)},
	}
	for index, want := range wants {
		message := errors[index].Message
		if !strings.Contains(message, "function "+want.name+" does not satisfy @noPanic") || !strings.Contains(message, want.kind) || !strings.Contains(message, "effect introduced at") {
			t.Fatalf("error %d = %q, want %s %s cause-aware violation", index, message, want.name, want.kind)
		}
	}
}
