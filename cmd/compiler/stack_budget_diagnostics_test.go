package main

import (
	"io"
	"math/big"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// TestStackBudgetDiagnosticTransport verifies the shared CLI occurrence adapter
// for explicitly supplied contracts without inventing project budget syntax.
// Rules: rules/analysis/stack_analysis.md — "Diagnostics";
// rules/tooling/diagnostics.md — §2(10) shared identity and §14 machine output.
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
		reporter := diagnosticReporter{format: diagnosticFormatJSON, output: io.Discard}
		reporter.semaDiagnostic(*value, value.Error())
		if len(reporter.occurrences) != 1 {
			t.Fatal("missing budget occurrence")
		}
		got := reporter.occurrences[0]
		definition, _ := diagnostics.Lookup(test.id)
		if got.ID == nil || *got.ID != test.id || got.Name == nil || *got.Name != definition.Name || got.Unregistered || got.Severity != diagnostics.SeverityError || len(got.Help) != 1 || got.Help[0].Text != value.Help || got.Primary == nil || got.Primary.Span.File != "worker.sec" {
			t.Fatal("stack budget diagnostic lost in CLI transport", got)
		}
	}
}
