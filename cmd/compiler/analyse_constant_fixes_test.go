package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestAnalyseConstantFixes preserves automatic-versus-intent/unsafe edit labels
// and exact constant replacements through default and explicit all-analysis CLI
// runs; advisory proofs never turn otherwise valid source into an error.
// Rules: rules/analysis/pitfall_analysis.md — "sec analyse", "Corrective actions",
// "Fix safety"; rules/compiler/compiler_analysis.md — §61.
func TestAnalyseConstantFixes(t *testing.T) {
	for _, options := range [][]string{nil, {"--all"}} {
		args := []string{"-test.run=^TestAnalyseCLIProcess$", "--", "analyse"}
		args = append(args, options...)
		args = append(args, "../../testdata/sema/pitfall_constant_fixes_valid.sec")
		command := exec.Command(os.Args[0], args...)
		command.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(output))
		}
		for _, want := range []string{
			"proven-fix: simplify the proven result to true",
			"proven-fix: simplify the proven result to false",
			"suggested-edit: test interval membership",
			"suggested-edit: replace with true only if evaluating the original expression may be omitted",
			"suggested-edit: replace with false only if evaluating the original expression may be omitted",
			"replacement: true", "replacement: false", "results: 0 errors",
		} {
			if !strings.Contains(string(output), want) {
				t.Fatalf("missing %q:\n%s", want, output)
			}
		}
	}
}
