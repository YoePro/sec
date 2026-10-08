package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// TestDefaultDiagnosticIdentityLSP verifies default diagnostic definitions
// override an advisory fallback and retain code, source span and repair help.
// Rules: rules/types/default_values.md — "Diagnostics", "LSP";
// rules/tooling/diagnostics.md — §15 Language Server Protocol.
func TestDefaultDiagnosticIdentityLSP(t *testing.T) {
	for _, id := range []string{diagnostics.DefaultCycle, diagnostics.BackendDefaultLeftUndefined} {
		failure := sema.Error{ID: id, Line: 1, Column: 1, Message: "default construction failed", Help: "inspect the default construction dependency"}
		value := semaDiagnostic(failure, 3, "")
		if value.Code != id || value.Severity != 1 || value.Range.Start.Line != 0 || value.Range.Start.Character != 0 || !strings.Contains(value.Message, failure.Help) {
			t.Fatal(value)
		}
		failure.Severity = diagnostics.SeverityWarning
		if value := semaDiagnostic(failure, 3, ""); value.Severity != 1 {
			t.Fatal("mandatory diagnostic was demoted", value)
		}
	}
	// Published configurable S diagnostics keep their effective source policy.
	failure := sema.Error{ID: diagnostics.IncompleteEnumSwitch, Severity: diagnostics.SeverityInformation}
	if value := semaDiagnostic(failure, 2, ""); value.Severity != 3 {
		t.Fatal("configurable policy was overridden", value)
	}
}
