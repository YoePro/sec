package diagnostics

import "testing"

// Every extended explanation belongs to a registered, active identity, names
// only registered related IDs and carries the § 13(6) core sections.
// Rules: rules/tooling/diagnostics.md — §§ 12(7), 13(6).
func TestExplanationsReferenceRegisteredDiagnostics(t *testing.T) {
	for _, id := range ExplainedIDs() {
		definition, ok := Lookup(id)
		if !ok || definition.Retired {
			t.Errorf("explanation for unregistered or retired %s", id)
			continue
		}
		explanation, _ := Explain(id)
		if explanation.Description == "" || explanation.Details == "" || len(explanation.Alternatives) == 0 {
			t.Errorf("%s: explanation lacks description, details or alternatives", id)
		}
		for _, related := range explanation.Related {
			if _, ok := Lookup(related); !ok || related == id {
				t.Errorf("%s: related %q is not another registered diagnostic", id, related)
			}
		}
	}
	if _, ok := Explain(UnreachableStatement); !ok {
		t.Fatal("S3001 has no extended explanation")
	}
	explanation, _ := Explain(UnreachableStatement)
	explanation.Related[0] = "mutated"
	if again, _ := Explain(UnreachableStatement); again.Related[0] == "mutated" {
		t.Fatal("Explain returned shared slices")
	}
}
