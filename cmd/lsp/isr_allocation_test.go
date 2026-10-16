package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestISRAllocationCoverageLSP consumes canonical proof states and source
// navigation for unsaved ISR bodies without deriving an editor allocation model.
// Rules: rules/memory/allocation.md — §§24(4),(6),28(4),29(4);
// rules/analysis/isr_analysis.md — §§25.2–25.3,31.3.
func TestISRAllocationCoverageLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/isr_allocation_coverage_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "isr.sec"))
	unproven, invalid := 0, 0
	for _, item := range analyze(uri, string(data)) {
		if item.Code != diagnostics.NoAllocViolation {
			continue
		}
		if item.Severity != 1 || item.Data == nil {
			t.Fatal(item)
		}
		switch item.Data.ProofState {
		case diagnostics.ProofUnproven:
			unproven++
			if !strings.Contains(item.Message, "cannot prove ISR @noAlloc") {
				t.Fatal(item)
			}
		case diagnostics.ProofInvalid:
			invalid++
		default:
			t.Fatal("missing proof state", item)
		}
		if len(item.RelatedInformation) == 0 || !strings.Contains(item.Message, "allocation policy root") {
			t.Fatal(item)
		}
		for _, related := range item.RelatedInformation {
			if related.Location.URI != uri {
				t.Fatal("lost unsaved source navigation", related)
			}
		}
	}
	if unproven != 8 || invalid != 1 {
		t.Fatal(unproven, invalid)
	}
}
