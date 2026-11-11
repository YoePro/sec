package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Copy-cost advice concerns proven legal copies, not move-only by-value
// transfers; escape/consumption requirements still block reference candidates.
// Rules: rules/memory/copy_move.md §22(1–6);
// rules/analysis/parameter_usage_analysis.md — Recommendation reasons, Candidate blockers.
func TestLargeCopyAdvicePreservesOwnership(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/large_copy_advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	byName := map[string]ParameterRecommendation{}
	for _, r := range a.ParameterUsageAnalysis().Recommendations() {
		byName[r.CallableName] = r
	}
	for _, name := range []string{"Read", "Forward"} {
		r := byName[name]
		if r.Status != "recommended" || !r.SizeKnown || r.EstimatedSizeBytes < 64 || r.Confidence != "strong" {
			t.Fatalf("missing safe copy advice: %+v", r)
		}
	}
	owner := byName["ObserveOwner"]
	if owner.Status == "recommended" || strings.Contains(strings.Join(owner.Reasons, " "), "AvoidCopyCost") {
		t.Fatalf("move-only value advertised as an expensive copy: %+v", owner)
	}
	for _, name := range []string{"Retain", "Consume", "Transfer"} {
		if byName[name].Status != "blocked" {
			t.Fatalf("ownership demand lost: %+v", byName[name])
		}
	}
	advisories := 0
	for _, w := range a.Warnings() {
		if w.ID == diagnostics.LargeValueParameter {
			advisories++
			if !strings.Contains(w.Help, "by-value copy is legal") || !strings.Contains(w.Help, "Estimated copy size:") {
				t.Fatalf("missing optional cost evidence: %+v", w)
			}
		}
		if w.ID == diagnostics.LargeValueParameter && strings.Contains(w.Message, "Owner") {
			t.Fatalf("move-only copy warning: %+v", w)
		}
	}
	if advisories != 2 {
		t.Fatalf("want two legal-copy advisories, got %d", advisories)
	}
}
