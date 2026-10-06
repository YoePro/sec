package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance",
// "Diagnostic ownership and coalescing"; rules/compiler/compiler_analysis.md — §61.
func TestAnalyseCanonicalIdiomGuidance(t *testing.T) {
	for _, options := range [][]string{nil, {"--all"}} {
		args := []string{"-test.run=^TestAnalyseCLIProcess$", "--", "analyse"}
		args = append(args, options...)
		args = append(args, "../../testdata/sema/pitfall_idioms_valid.sec")
		command := exec.Command(os.Args[0], args...)
		command.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal("idiom guidance must not reject valid source", err, string(output))
		}
		for _, want := range []string{
			"canonical idiom: range-membership", "replacement: value in 1..<9",
			"canonical idiom: half-open-traversal", "replacement: uint(0)..<values.Len",
			"proven-fix: use canonical range membership",
			"suggested-edit: use the half-open traversal, which is empty for an empty collection",
			"suggested-edit: traverse every element with ..<Len",
			"results: 0 errors",
		} {
			if !strings.Contains(string(output), want) {
				t.Fatalf("missing %q in report:\n%s", want, output)
			}
		}
		if strings.Contains(string(output), "strict") {
			t.Fatal("guidance introduced strict mode", string(output))
		}
	}
}
