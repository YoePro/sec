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
