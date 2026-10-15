package main

import (
	"strings"
	"testing"
)

// Inclusive cross-collection loops retain endpoint-sensitive proof evidence
// in CLI reports; intent advice does not make an unknown traversal invalid.
// Rules: rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection", "sec analyse".
func TestAnalyseInclusiveBoundSource(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "../../testdata/sema/pitfall_inclusive_bound_source.sec")
	rule := "pitfall.collection.wrong-bound-source"
	if code != 0 || strings.Count(stdout, "suppressed "+rule) != 7 || strings.Count(stdout, rule) != 20 {
		t.Fatal(code, stdout, stderr)
	}
}
