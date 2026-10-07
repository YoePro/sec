package diagnostics

import "testing"

// TestIndexBoundsDefinition protects ordinary mandatory bounds ownership;
// analysis rule names never become diagnostic IDs or configurable warnings.
// Rules: rules/tooling/diagnostics.md — §§4–7;
// rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing".
func TestIndexBoundsDefinition(t *testing.T) {
	d, ok := Lookup(IndexOutOfBounds)
	if !ok || d.ID != "S1134" || d.Name != "bounds.index-out-of-bounds" || d.Family != "bounds" || !d.Mandatory || d.DefaultSeverity != SeverityError {
		t.Fatal(d)
	}
}
