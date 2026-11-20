package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// All frontend routes transport Sema's exact borrow/capability distinction;
// no CLI wrapper accepts a call with missing element-level reference proof.
// Rules: rules/declarations/spread.md §4.2; rules/tooling/diagnostics.md §§9,17.
func TestSpreadBorrowAuditCLI(t *testing.T) {
	for _, command := range []string{"check", "emit-ir"} {
		args := []string{command, "../../testdata/sema/spread_borrow_audit_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", "-")
		}
		stdout, stderr, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, stderr)
		if code != 3 || stdout != "" || len(doc.Occurrences) != 8 {
			t.Fatalf("%s: %d %s %s", command, code, stdout, stderr)
		}
		want := []string{"S1161", "S1239", "S1239", "S1239", "S1161", "S1161", "S1239", "S1239"}
		for i, item := range doc.Occurrences {
			if item.ID == nil || *item.ID != want[i] || item.Primary == nil || len(item.Help) == 0 {
				t.Fatalf("lost Sema diagnostic: %+v", item)
			}
			if want[i] == "S1239" && (!diagnostics.IsImplementationCapability(*item.ID) || item.Name == nil || *item.Name != "compiler.capability-unavailable" || item.Arguments["proof_state"] != "") {
				t.Fatalf("lost capability class: %+v", item)
			}
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/spread_borrow_audit_valid.sec")
	if code != 0 {
		t.Fatalf("valid controls: %d %s", code, stderr)
	}
}
