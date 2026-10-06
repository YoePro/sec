package main

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// TestStackBudgetDiagnosticTransport verifies shared LSP identity, mandatory
// severity and explanatory help for externally supplied active budget facts.
// Rules: rules/analysis/stack_analysis.md — "Diagnostics";
// rules/tooling/lsp.md — "Shared diagnostic model".
func TestStackBudgetDiagnosticTransport(t *testing.T) {
	budget, err := sema.NewStackBudget("thread", sema.StackMeasurementMachine, big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	exact, _ := sema.NewExactStackBound(big.NewInt(101))
	for _, test := range []struct {
		bound sema.StackBound
		id    string
	}{
		{exact, diagnostics.StackBudgetExceeded},
		{sema.UnknownStackBound(), diagnostics.StackBudgetProofUnavailable},
		{sema.UnboundedStackBound(), diagnostics.StackBudgetUnboundedDemand},
	} {
		value, err := sema.DiagnoseStackBudget(&budget, "thread", sema.StackMeasurementMachine, test.bound,
			lexer.Token{File: "worker.sec", Line: 1, Column: 1, EndLine: 1, EndColumn: 7}, sema.StackEvidence{})
		if err != nil || value == nil {
			t.Fatal(value, err)
		}
		got := semaDiagnostic(*value, 3, "Worker")
		if got.Code != test.id || got.Severity != 1 || got.Range.Start.Character != 0 || got.Range.End.Character != 6 || !strings.Contains(got.Message, value.Help) {
			t.Fatal("stack budget diagnostic lost in LSP transport", got)
		}
	}
}
