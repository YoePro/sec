package sema

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestStackBudgetDiagnostics verifies distinct proof outcomes under explicit
// contracts and source-local mandatory identities, without declaring overflow
// certain merely because a proof failed.
// Rules: rules/analysis/stack_analysis.md — "Diagnostics", "Budget validation",
// "No-budget behavior", and "Machine-level revalidation";
// rules/tooling/diagnostics.md — §5 and §7.
func TestStackBudgetDiagnostics(t *testing.T) {
	source := lexer.Token{File: "worker.sec", Lexeme: "Worker", Line: 2, Column: 4, EndLine: 2, EndColumn: 10}
	for _, level := range []StackMeasurementLevel{StackMeasurementSemantic, StackMeasurementMachine} {
		budget, err := NewStackBudget("worker-stack", level, big.NewInt(8192))
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			kind    StackBoundKind
			count   int64
			id      string
			message string
		}{
			{StackBoundExact, 0, "", ""},
			{StackBoundExact, 8192, "", ""},
			{StackBoundUpperBound, 8192, "", ""},
			{StackBoundExact, 9216, diagnostics.StackBudgetExceeded, "required 9216 B, available 8192 B"},
			{StackBoundUpperBound, 9216, diagnostics.StackBudgetProofUnavailable, "verified upper bound 9216 B, available 8192 B"},
			{StackBoundUnknown, 0, diagnostics.StackBudgetProofUnavailable, "finite " + string(level) + " stack bound could not be proven"},
			{StackBoundUnbounded, 0, diagnostics.StackBudgetUnboundedDemand, "proven unbounded " + string(level) + " stack demand"},
		} {
			bound := UnknownStackBound()
			switch test.kind {
			case StackBoundExact:
				bound, err = NewExactStackBound(big.NewInt(test.count))
			case StackBoundUpperBound:
				bound, err = NewUpperStackBound(big.NewInt(test.count))
			case StackBoundUnbounded:
				bound = UnboundedStackBound()
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := DiagnoseStackBudget(&budget, budget.Domain(), level, bound, source, StackEvidence{})
			if err != nil {
				t.Fatal(err)
			}
			if test.id == "" {
				if got != nil {
					t.Fatal("satisfied budget rejected", got)
				}
				continue
			}
			if got == nil || got.ID != test.id || got.Severity != diagnostics.SeverityError || got.Help == "" || !strings.Contains(got.Message, test.message) || !strings.Contains(got.Message, string(level)) || !strings.Contains(got.Message, "worker-stack") {
				t.Fatal("incorrect proof diagnostic", got)
			}
			if got.File != source.File || got.Line != source.Line || got.Column != source.Column || got.EndLine != source.EndLine || got.EndColumn != source.EndColumn {
				t.Fatal("contract/root source span lost", got)
			}
			definition, registered := diagnostics.Lookup(got.ID)
			if !registered || !definition.Mandatory || definition.Retired || definition.DefaultSeverity != diagnostics.SeverityError || definition.Family != "stack" {
				t.Fatal("budget proof diagnostic unregistered or demotable", definition)
			}
			if test.kind == StackBoundUpperBound && !strings.Contains(got.Help, "does not prove actual excess") {
				t.Fatal("upper bound promoted to proven excess", got)
			}
			if test.kind == StackBoundUnknown && !strings.Contains(got.Help, "Unknown does not prove stack overflow") {
				t.Fatal("uncertainty promoted to overflow", got)
			}
			if inactive, err := DiagnoseStackBudget(nil, budget.Domain(), level, bound, source, StackEvidence{}); err != nil || inactive != nil {
				t.Fatal("absent contract activated policy", inactive, err)
			}
		}
		wrongLevel := StackMeasurementMachine
		if level == wrongLevel {
			wrongLevel = StackMeasurementSemantic
		}
		for _, mismatch := range []struct {
			domain string
			level  StackMeasurementLevel
		}{
			{"other", level}, {budget.Domain(), wrongLevel}, {budget.Domain(), ""},
		} {
			if got, err := DiagnoseStackBudget(&budget, mismatch.domain, mismatch.level, UnknownStackBound(), source, StackEvidence{}); err == nil || got != nil {
				t.Fatal("scope mismatch became a language diagnostic", got, err)
			}
		}
	}
	if got, err := DiagnoseStackBudget(&StackBudget{}, "", "", UnknownStackBound(), source, StackEvidence{}); err == nil || got != nil {
		t.Fatal("uninitialized contract became a diagnostic", got, err)
	}
}

// TestStackBudgetDiagnosticEvidence retains a deterministic uncertainty source,
// immutable evidence and arbitrary-precision counts without using known prefix
// bytes to satisfy an unknown overall requirement.
// Rules: rules/analysis/stack_analysis.md — "Partial information", "Diagnostics",
// "Stack cause paths", and "Determinism".
func TestStackBudgetDiagnosticEvidence(t *testing.T) {
	maximum := new(big.Int).Lsh(big.NewInt(1), 128)
	budget, err := NewStackBudget("thread", StackMeasurementMachine, maximum)
	if err != nil {
		t.Fatal(err)
	}
	bound, _ := NewExactStackBound(new(big.Int).Add(maximum, big.NewInt(1)))
	excess, err := DiagnoseStackBudget(&budget, "thread", StackMeasurementMachine, bound, lexer.Token{}, StackEvidence{})
	if err != nil || excess == nil || !strings.Contains(excess.Message, maximum.String()) || !strings.Contains(excess.Message, "340282366920938463463374607431768211457") {
		t.Fatal("diagnostic count overflow", excess, err)
	}
	frame, _ := NewExactStackBound(big.NewInt(1536))
	evidence := StackEvidence{KnownPrefix: []StackFrameContribution{{Frame: frame}}, UnknownCauses: []StackCauseStep{
		{Detail: "later runtime boundary", Source: lexer.Token{File: "z.sec", Line: 10, Column: 3}},
		{Detail: "callback has no verified stack bound", Source: lexer.Token{File: "a.sec", Line: 4, Column: 7}},
	}}
	before := cloneStackEvidence(evidence)
	got, err := DiagnoseStackBudget(&budget, "thread", StackMeasurementMachine, UnknownStackBound(), lexer.Token{}, evidence)
	if err != nil || got == nil || got.ID != diagnostics.StackBudgetProofUnavailable || got.PreviousFile != "a.sec" || got.PreviousLine != 4 || got.PreviousColumn != 7 || got.RelatedLabel != "stack proof boundary" || !strings.Contains(got.Help, evidence.UnknownCauses[1].Detail) {
		t.Fatal("uncertainty source lost", got, err)
	}
	if !reflect.DeepEqual(before, evidence) {
		t.Fatal("diagnostic changed producer evidence")
	}
	evidence.UnknownCauses[0], evidence.UnknownCauses[1] = evidence.UnknownCauses[1], evidence.UnknownCauses[0]
	reversed, err := DiagnoseStackBudget(&budget, "thread", StackMeasurementMachine, UnknownStackBound(), lexer.Token{}, evidence)
	if err != nil || !reflect.DeepEqual(got, reversed) {
		t.Fatal("representative cause order unstable", reversed, err)
	}
	unbounded, err := DiagnoseStackBudget(&budget, "thread", StackMeasurementMachine, UnboundedStackBound(), lexer.Token{}, evidence)
	if err != nil || unbounded == nil || unbounded.PreviousLine != 0 || strings.Contains(unbounded.Help, "callback") {
		t.Fatal("unknown cause became unbounded proof cause", unbounded, err)
	}
}
