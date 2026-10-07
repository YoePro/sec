package diagnostics

import "testing"

// TestPitfallAdvisoryDefinition keeps analysis rule identities separate from
// the configurable public diagnostic identity and mandatory semantic owners.
// Rules: rules/tooling/diagnostics.md — §§5,18;
// rules/analysis/pitfall_analysis.md — "Diagnostic ownership and coalescing".
func TestPitfallAdvisoryDefinition(t *testing.T) {
	d, ok := Lookup(PitfallAdvisory)
	if !ok || d.ID != "A2004" || d.Name != "suspicious.pitfall" || d.Family != "suspicious" || d.Mandatory || d.Retired || d.DefaultSeverity != SeverityInformation {
		t.Fatal(d, ok)
	}
}
