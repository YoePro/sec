package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The CLI exposes final-copy evidence and the explicit manual replacement
// through the existing pitfall report; the recommendation is not a source error.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use", "sec analyse".
func TestAnalyseLastUseArgumentMove(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestAnalyseCLIProcess$", "--", "analyse", "pitfalls", "--verbose", "../../testdata/sema/pitfall_move/arguments.sec")
	cmd.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	for _, want := range []string{"pitfall.ownership.last-use-argument-copy", "replacement: <-files", "suggested-edit: Consider moving files", "estimated aggregate value copy", "results: 0 errors"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(string(output), "proven-fix: Consider moving files") {
		t.Fatal("ownership change advertised as automatic fix")
	}
}
