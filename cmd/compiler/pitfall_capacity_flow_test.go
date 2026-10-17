package main

import (
	"strings"
	"testing"
)

// Deep reports retain suppression witnesses while mutation and execution
// boundaries keep their advisory. No heuristic becomes a validity error.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Evidence against a finding", "sec analyse".
func TestAnalyseCapacityProofFlow(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/pitfall_capacity_flow.sec")
	if code != 0 {
		t.Fatal(code, stderr, stdout)
	}
	rule := "pitfall.bounds.capacity-as-length"
	if strings.Count(stdout, "suppressed "+rule) != 18 || strings.Count(stdout, rule) != 37 {
		t.Fatal(stdout, stderr)
	}
}

// Manual while traversals retain per-access proof evidence in Deep reports.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "sec analyse", "Evidence against a finding".
func TestAnalyseCapacityWhile(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/pitfall_capacity_while.sec")
	if code != 0 {
		t.Fatal(code, stderr, stdout)
	}
	rule := "pitfall.bounds.capacity-as-length"
	if strings.Count(stdout, "suppressed "+rule) != 10 || strings.Count(stdout, rule) != 15 {
		t.Fatal(stdout, stderr)
	}
}

// Short-circuit witnesses are explicit suppressions in Deep reports; a late
// check remains advisory and unsupported relations remain unknown.
// Rules: rules/analysis/pitfall_analysis.md — "sec analyse", "Suppressing evidence".
func TestAnalyseCapacityExpressions(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/pitfall_capacity_expressions.sec")
	if code != 0 {
		t.Fatal(code, stderr, stdout)
	}
	rule := "pitfall.bounds.capacity-as-length"
	if strings.Count(stdout, "suppressed "+rule) != 15 || strings.Count(stdout, rule) != 18 {
		t.Fatal(stdout, stderr)
	}
}

// Equality never turns an exact Capacity index into a valid live-element index.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Diagnostic ownership and coalescing".
func TestAnalyseCapacityShortCircuitInvalid(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/pitfall_capacity_shortcircuit_invalid.sec")
	if code != 3 || !strings.Contains(stdout, "error pitfall.bounds.capacity-as-length") || !strings.Contains(stdout, "proven-invalid") || strings.Contains(stdout, "suppressed pitfall.bounds.capacity-as-length") {
		t.Fatal(code, stdout, stderr)
	}
}
