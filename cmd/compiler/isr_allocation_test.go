package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestISRAllocationCoverageCLI keeps canonical ISR proof states and cause
// navigation in both compilation and analysis JSON output.
// Rules: rules/memory/allocation.md — §§24(4),(6),28(4),29(4);
// rules/analysis/isr_analysis.md — §§25.2–25.3,32.
func TestISRAllocationCoverageCLI(t *testing.T) {
	for _, command := range []string{"check", "analyse"} {
		_, stderr, code := runCLIForDiagnostics(t, command, "../../testdata/sema/isr_allocation_coverage_invalid.sec", "--diagnostic-format=json")
		if code != 3 {
			t.Fatal(command, code, stderr)
		}
		doc := decodeOccurrenceDocument(t, stderr)
		unproven, invalid := 0, 0
		for _, item := range doc.Occurrences {
			if item.ID == nil || *item.ID != diagnostics.NoAllocViolation {
				continue
			}
			switch item.Arguments["proof_state"] {
			case string(diagnostics.ProofUnproven):
				unproven++
				if !strings.Contains(item.Message.Text, "cannot prove ISR @noAlloc") {
					t.Fatal(item)
				}
			case string(diagnostics.ProofInvalid):
				invalid++
			default:
				t.Fatal("missing proof state", item)
			}
			if len(item.Notes) < 3 || len(item.Related) == 0 {
				t.Fatal("missing canonical cause", item)
			}
			for _, note := range item.Notes {
				if note.Arguments["incomplete"] != "false" {
					t.Fatal(note)
				}
			}
		}
		if unproven != 8 || invalid != 1 {
			t.Fatal(command, unproven, invalid, doc)
		}
	}
}
