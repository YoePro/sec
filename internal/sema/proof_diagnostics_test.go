package sema

import (
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestIteratorProofDiagnosticsAreDetached verifies a generic pipeline adapter
// cannot mutate stored issues or turn absent/unknown evidence into Invalid.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §58(3);
// rules/compiler/compiler_pipeline.md — §33(3).
func TestIteratorProofDiagnosticsAreDetached(t *testing.T) {
	failure := &IteratorLoweringReadinessError{Issues: []IteratorLoweringIssue{
		{Source: lexer.Token{File: "source.sec", Line: 2, Column: 3, EndLine: 2, EndColumn: 6}, Classification: diagnostics.ProofInvalid, Reason: "violated conformance"},
		{Classification: diagnostics.ProofUnproven, Reason: "missing snapshot"},
		{Classification: "", Reason: "absent classification"},
	}}
	values := failure.SemanticDiagnostics()
	if len(values) != 3 || values[0].ProofState != diagnostics.ProofInvalid || values[0].ID != diagnostics.RequiredAnalysisInvalid || values[0].File != "source.sec" || values[0].EndColumn != 6 {
		t.Fatal(values)
	}
	for _, value := range values[1:] {
		if value.ProofState != diagnostics.ProofUnproven || value.ID != diagnostics.RequiredProofUnavailable || value.Line != 0 {
			t.Fatal(value)
		}
	}
	values[0].ProofState = diagnostics.ProofUnproven
	values[0].Message = "changed"
	again := failure.SemanticDiagnostics()
	if again[0].ProofState != diagnostics.ProofInvalid || again[0].Message == "changed" || failure.Issues[0].Reason != "violated conformance" {
		t.Fatal("published diagnostics alias evidence", again)
	}
	for _, value := range again {
		definition, ok := diagnostics.Lookup(value.ID)
		if !ok || !definition.Mandatory || definition.DefaultSeverity != diagnostics.SeverityError {
			t.Fatal(value)
		}
	}
}
