package diagnostics

import "testing"

// TestEscapeDiagnosticDefinitions pins the distinct escape meanings and their
// mandatory severity in the central registry.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories";
// rules/tooling/diagnostics.md — §§4–7.
func TestEscapeDiagnosticDefinitions(t *testing.T) {
	for id, name := range map[string]string{
		"S1128": "escape.local-storage",
		"S1129": "escape.outer-place",
		"S1130": "escape.match-payload",
		"S1131": "escape.closure-capture",
		"S1132": "escape.provenance-unknown",
		"S1133": "escape.variadic-pack",
	} {
		d, ok := Lookup(id)
		if !ok || d.ID != id || d.Name != name || d.Family != "escape" || d.DefaultSeverity != SeverityError || !d.Mandatory {
			t.Fatal(id, d)
		}
	}
}
