package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// rules/errors/panic.md § 15.2 requires bool exactly and defines no truthiness
// conversion for assertion conditions.
func TestAssertConditionMustBeBool(t *testing.T) {
	assertSemaErrors(t, analyzeSourceRaw(t, `
module main

fn Good(ready: bool) void {
	assert ready
	assert ready, "still ready"
}

fn Bad(value: int) void {
	assert value
}
`), []string{
		"assert condition must be bool, got int at 10:9",
	})
}

// rules/errors/panic.md §§ 17 and 21 require explicit panic to be
// non-returning and visible to @noPanic verification.
func TestExplicitPanicTerminatesAndRecordsEffect(t *testing.T) {
	analyzer, validErrors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Fail() int {
	panic "failure"
}
`)
	assertSemaErrors(t, validErrors, nil)
	summary := analyzer.CallGraph().EffectSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "Fail"))
	if len(summary.DirectEffects) != 1 || len(summary.DirectEffects[0].PanicReasonIDs) != 1 ||
		summary.DirectEffects[0].PanicReasonIDs[0] != diagnostics.PanicReasonExplicitPanic {
		t.Fatalf("explicit panic effects = %+v", summary.DirectEffects)
	}

	errors := analyzeSourceRaw(t, `
module main

@noPanic
fn Fail() void {
	panic "failure"
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "function Fail does not satisfy @noPanic") ||
		!strings.Contains(errors[0].Message, string(EffectMayPanicExplicit)) {
		t.Fatalf("errors = %v, want explicit-panic @noPanic violation", errors)
	}
}
