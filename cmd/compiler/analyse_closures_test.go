package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The registered selector reaches the real producer, preserves clean silence,
// isolates verbose facts, supports focus/JSON, rejects aliases and retains
// mandatory language diagnostics. A repeated run preserves fact ordering.
// Rules: rules/tooling/cli.md — § 3(2), § 5.2–§ 5.6, § 6;
// rules/analysis/closure_analysis.md — "sec analyse".
func TestAnalyseClosures(t *testing.T) {
	const fixture = "../../testdata/tooling/analyse_cli/closures.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "closures", fixture)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean closure analysis: %d %q %q", code, stdout, stderr)
	}
	verbose, stderr, code := runCLIForDiagnostics(t, "analyse", "closures", "--verbose", fixture)
	if code != 0 || stderr != "" {
		t.Fatalf("closure facts: %d %s %s", code, verbose, stderr)
	}
	for _, want := range []string{"closures:", "creation at", "environment=true, captures=1", "environment=false, captures=0", "capture seed: mode=copy-capture, transfer=copy", "capture other:", "call at", "analyses: closures"} {
		if !strings.Contains(verbose, want) {
			t.Errorf("missing %q:\n%s", want, verbose)
		}
	}
	for _, unwanted := range []string{"call graph:", "parameter usage:", "pitfalls:"} {
		if strings.Contains(verbose, unwanted) {
			t.Errorf("closure selection includes %q", unwanted)
		}
	}
	again, _, _ := runCLIForDiagnostics(t, "analyse", "closures", "--verbose", fixture)
	if again != verbose {
		t.Fatal("closure report ordering changed across processes")
	}
	focused, _, code := runCLIForDiagnostics(t, "analyse", "closures", "--verbose", "-f", "Work", fixture)
	if code != 0 || !strings.Contains(focused, "capture seed:") || strings.Contains(focused, "capture other:") {
		t.Fatalf("closure focus: %d %s", code, focused)
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "analyse", "closures", "--json", fixture)
	var report analysisReportDocument
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || code != 0 || stderr != "" || len(report.Analyses) != 1 ||
		report.Analyses[0].Name != "closures" || report.Analyses[0].Status != "executed" || !report.Analyses[0].Complete {
		t.Fatalf("JSON selection: %d %v %s %s", code, err, stdout, stderr)
	}
	stdout, _, code = runCLIForDiagnostics(t, "analyse", "closures", analyseAdvisoryFixture)
	if code != 0 || stdout != "" {
		t.Fatalf("closure selection leaked other analyses: %d %s", code, stdout)
	}
	_, stderr, code = runCLIForDiagnostics(t, "analyse", "closure", fixture)
	if code != 1 || !strings.Contains(stderr, `did you mean "closures"`) {
		t.Fatalf("unregistered alias accepted: %d %s", code, stderr)
	}
	stdout, _, code = runCLIForDiagnostics(t, "analyse", "closures", "--json", analyseRejectedFixture)
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || code == 0 || report.Status != "rejected" || len(report.Diagnostics) == 0 {
		t.Fatalf("mandatory diagnostics suppressed: %d %v %s", code, err, stdout)
	}
}
