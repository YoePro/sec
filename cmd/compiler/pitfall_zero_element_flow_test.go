package main

import (
	"strings"
	"testing"
)

// Mutation invalidates dominating zero-handling evidence in CLI reports;
// intent advice remains optional and retains suppression evidence.
// Rules: rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory", "sec analyse".
func TestAnalyseZeroElementFlow(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "../../testdata/sema/pitfall_zero_element_flow.sec")
	rule := "pitfall.bounds.skipped-zero"
	if code != 0 || strings.Count(stdout, "suppressed "+rule) != 14 || strings.Count(stdout, rule) != 30 {
		t.Fatal(code, stdout, stderr)
	}
}
