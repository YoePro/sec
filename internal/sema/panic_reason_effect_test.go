package sema

import (
	"reflect"
	"testing"

	"sec/internal/diagnostics"
)

func TestCheckedIntegerEffectsCarryCanonicalPanicReasonSets(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn Negate(value: int) int { return -value }
fn Add(left: int, right: int) int { return left + right }
fn DivideSigned(left: int, right: int) int { return left / right }
fn DivideUnsigned(left: uint, right: uint) uint { return left / right }
fn RemainderSigned(left: int, right: int) int { return left % right }
fn ShiftLeftSigned(left: int, right: int) int { return left << right }
fn ShiftLeftUnsigned(left: uint, right: int) uint { return left << right }
fn ShiftRightSigned(left: int, right: int) int { return left >> right }
`)
	assertSemaErrors(t, errors, nil)

	want := map[string][]diagnostics.PanicReasonID{
		"Negate":            {diagnostics.PanicReasonArithmeticOverflow},
		"Add":               {diagnostics.PanicReasonArithmeticOverflow},
		"DivideSigned":      {diagnostics.PanicReasonDivisionByZero, diagnostics.PanicReasonArithmeticOverflow},
		"DivideUnsigned":    {diagnostics.PanicReasonDivisionByZero},
		"RemainderSigned":   {diagnostics.PanicReasonDivisionByZero, diagnostics.PanicReasonArithmeticOverflow},
		"ShiftLeftSigned":   {diagnostics.PanicReasonInvalidShift, diagnostics.PanicReasonArithmeticOverflow},
		"ShiftLeftUnsigned": {diagnostics.PanicReasonInvalidShift},
		"ShiftRightSigned":  {diagnostics.PanicReasonInvalidShift},
	}
	graph := analyzer.CallGraph()
	for name, reasons := range want {
		summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, name))
		if len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != EffectMayPanicArithmetic ||
			!reflect.DeepEqual(summary.DirectEffects[0].PanicReasonIDs, reasons) {
			t.Errorf("%s effects = %+v, want reasons %v", name, summary.DirectEffects, reasons)
		}
	}

	summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, "DivideSigned"))
	summary.DirectEffects[0].PanicReasonIDs[0] = diagnostics.PanicReasonForeignAbort
	again := graph.EffectSummary(callGraphNodeIDByName(t, graph, "DivideSigned"))
	if again.DirectEffects[0].PanicReasonIDs[0] != diagnostics.PanicReasonDivisionByZero {
		t.Fatal("effect summary exposed mutable panic-reason storage")
	}
}
