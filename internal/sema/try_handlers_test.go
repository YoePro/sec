package sema

import (
	"os"
	"strings"
	"testing"
)

// Partial handlers propagate unmatched failures through a compatible Result
// return, where guards and block recovery values are accepted, and void
// success handlers may complete normally. The resolved plan records residual
// propagation, guarded handlers, and block values explicitly.
//
// Rules:
//   - rules/errors/errorhandling.md — §16 "Partial handlers and implicit propagation", §19 "Guards"
//   - rules/errors/errorhandling.md — §21 "Recovery values", §21.1 "Block handler result position", §21.2 "void success"
func TestTryHandlersPartialGuardsAndBlockValues(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/try_handlers_partial_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(input))
	if len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
	var residual, guarded, blockValues int
	for _, plan := range analyzer.resolvedTryPlans {
		if plan.ResidualPropagates {
			residual++
			if plan.Exhaustive || plan.EnclosingResultType.Kind != ResultType {
				t.Fatalf("residual plan = %+v", plan)
			}
		}
		for _, handler := range plan.Handlers {
			if handler.Guarded {
				guarded++
			}
			if handler.BlockValue {
				blockValues++
			}
		}
	}
	if residual != 2 || guarded != 1 || blockValues != 1 {
		t.Fatalf("residual = %d, guarded = %d, block values = %d; want 2, 1, 1", residual, guarded, blockValues)
	}
}

// Residual failures that cannot propagate, guard-only coverage, non-bool
// guards, and wrongly typed block recovery values are rejected with mentor
// diagnostics that name the unhandled failures.
//
// Rules:
//   - rules/errors/errorhandling.md — §12.1, §16, §19, §21.1, §30 "Diagnostics must act as a mentor"
func TestTryHandlerPartialDiagnostics(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/try_handlers_partial_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line    int
		message string
	}{
		{26, "try handlers leave IOError.PermissionDenied, IOError.Busy unhandled; they would propagate with return Err, but this function returns int"},
		{37, "but this function returns Result[int, OtherError]; add Err(_) => ... or map IOError to OtherError"},
		{44, "try handlers leave IOError.NotFound, IOError.PermissionDenied, IOError.Busy unhandled"},
		{52, "try handler guard must be bool, got int"},
		{62, "try handler must produce int, got string"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		if errors[i].Line != want.line || !strings.Contains(errors[i].Message, want.message) {
			t.Fatalf("error %d = %+v, want %q at line %d", i, errors[i], want.message, want.line)
		}
	}
}

// A guard runs before its handler is selected, so it may borrow but must not
// consume a move-only payload binding.
//
// Rules:
//   - rules/errors/errorhandling.md — §20 "Handler ownership and guards"
func TestTryHandlerGuardMustNotConsumeBinding(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/try_handler_guard_consume_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	if len(errors) != 1 || errors[0].Line != 30 || !strings.Contains(errors[0].Message, "try handler guard must not consume e before the handler is selected") {
		t.Fatalf("errors = %+v, want one guard-consumption diagnostic", errors)
	}
}
