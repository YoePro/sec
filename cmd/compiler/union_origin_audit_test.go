package main

import "testing"

// TestUnionOriginAuditCLI requires identical referent diagnostics for payload
// forms and rejects invalid IR emission before producing an artifact.
// Rules: rules/memory/lifetime_analysis.md §§14,28;
// rules/tooling/diagnostics.md §§9,17.
func TestUnionOriginAuditCLI(t *testing.T) {
	for _, command := range []string{"check", "emit-ir"} {
		args := []string{command, "../../testdata/sema/union_origin_audit_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", "-")
		}
		out, stderr, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, stderr)
		if code != 3 || out != "" || len(doc.Occurrences) != 12 {
			t.Fatalf("%s: %d %s %s", command, code, out, stderr)
		}
		for i, occ := range doc.Occurrences {
			if occ.ID == nil || occ.Primary == nil {
				t.Fatalf("lost origin diagnostic: %+v", occ)
			}
			if i%2 == 1 && *occ.ID != *doc.Occurrences[i-1].ID {
				t.Fatal("constructor forms disagree")
			}
		}
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/union_origin_audit_valid.sec"); code != 0 {
		t.Fatalf("valid controls: %d %s", code, stderr)
	}
}
