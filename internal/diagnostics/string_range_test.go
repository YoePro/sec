package diagnostics

import "testing"

// Range errors are mandatory compiler diagnostics shared with the editor.
// Rules: rules/tooling/diagnostics.md — §§4–7; collections.md — §7.10 (MD-126).
func TestStringRangeDefinitions(t *testing.T) {
	for id, name := range map[string]string{InvalidRangeSource: "bounds.invalid-range-source", InvalidSliceRange: "bounds.invalid-slice-range"} {
		d, ok := Lookup(id)
		if !ok || d.Name != name || d.Family != "bounds" || !d.Mandatory || d.DefaultSeverity != SeverityError {
			t.Fatal(d)
		}
	}
}
