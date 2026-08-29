package sema

import "testing"

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
}
