package main

import (
	"bytes"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// TestDefaultDiagnosticIdentityTransport verifies registered occurrences can
// cross compiler reporting without losing identity, guidance or source context.
// It tests transport; it does not claim these failures are currently produced.
// Rules: rules/types/default_values.md — "Diagnostics";
// rules/tooling/diagnostics.md — CLI and shared diagnostic model.
func TestDefaultDiagnosticIdentityTransport(t *testing.T) {
	for _, id := range []string{diagnostics.DefaultCycle, diagnostics.BackendDefaultLeftUndefined} {
		failure := sema.Error{ID: id, File: "source.sec", Line: 3, Column: 6, Message: "default construction failed", Help: "inspect the default construction dependency"}
		var human, json bytes.Buffer
		(&diagnosticReporter{format: diagnosticFormatHuman, output: &human}).pipelineError("lowering", failure)
		reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &json}
		reporter.pipelineError("lowering", failure)
		reporter.finish()
		doc := decodeOccurrenceDocument(t, json.String())
		if doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
			t.Fatal(doc)
		}
		value := doc.Occurrences[0]
		definition, _ := diagnostics.Lookup(id)
		if value.ID == nil || *value.ID != id || value.Name == nil || *value.Name != definition.Name || value.Unregistered || value.Severity != diagnostics.SeverityError || value.Primary == nil || value.Primary.Span.Start.Line != 3 || len(value.Help) != 1 {
			t.Fatal(value)
		}
		if !strings.Contains(human.String(), id) || !strings.Contains(human.String(), failure.Help) {
			t.Fatal(human.String())
		}
	}
}
