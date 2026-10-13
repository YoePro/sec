package main

import (
	"strings"
	"testing"
)

// TestAnalyseNonEmptyProofs preserves explicit suppression evidence in Deep
// reports while retaining advisories where length-changing execution intervenes.
// Rules: rules/analysis/pitfall_analysis.md — Final-element access requires
// non-empty proof, sec analyse, Evidence against a finding.
func TestAnalyseNonEmptyProofs(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "../../testdata/sema/pitfall_nonempty_flow.sec")
	if code != 0 {
		t.Fatal(code, stderr, stdout)
	}
	rule := "pitfall.bounds.final-element-needs-nonempty"
	if strings.Count(stdout, "suppressed "+rule) != 11 || strings.Count(stdout, rule) != 18 {
		t.Fatal(stdout, stderr)
	}
	if !strings.Contains(stdout, "the resolved collection type proves a positive minimum length") {
		t.Fatal(stdout)
	}
}
