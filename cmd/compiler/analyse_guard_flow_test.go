package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestAnalyseGuardFlow transports later/nested path evidence through the shared
// analysis report without promoting advisory findings to semantic errors.
// Rules: rules/analysis/pitfall_analysis.md — "CLI integration", "Guard checks the wrong value",
// "Safety check without control transfer", "Diagnostic ownership and coalescing".
func TestAnalyseGuardFlow(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestAnalyseCLIProcess$", "--", "analyse", "../../testdata/sema/pitfall_guard_flow_valid.sec")
	command.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	for _, want := range []string{
		"pitfall.control-flow.wrong-guard-subject",
		"pitfall.control-flow.check-without-transfer",
		"a later reachable access uses the same unchanged collection and index without a protecting guard",
		"guard the index and collection that are accessed",
		"results: 0 errors",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %q in report:\n%s", want, output)
		}
	}
}
