package main

import (
	"strings"
	"testing"
)

// Verbose analysis consumes canonical yield roles and preserves exact function
// focus without turning a physical scheduler hint into a cancellation point.
// Rules: rules/concurrency/scheduling.md §§15,55; rules/tooling/cli.md §§5.3–5.4.
func TestAnalyseSchedulingPoints(t *testing.T) {
	const fixture = "../../testdata/sema/scheduling_point_facts_valid.sec"
	out, errText, code := runCLIForDiagnostics(t, "analyse", "--verbose", fixture)
	if code != 0 || errText != "" || !strings.Contains(out, "scheduling points:") || strings.Count(out, "logical task scheduling and cancellation point at ") != 1 || strings.Count(out, "physical thread scheduler hint at ") != 2 {
		t.Fatal(code, out, errText)
	}
	out, errText, code = runCLIForDiagnostics(t, "analyse", "--verbose", "-f", "Work", fixture)
	if code != 0 || strings.Count(out, "logical task scheduling and cancellation point at ") != 1 || strings.Count(out, "physical thread scheduler hint at ") != 1 {
		t.Fatal(code, out, errText)
	}
	out, errText, code = runCLIForDiagnostics(t, "analyse", fixture)
	if code != 0 || strings.Contains(out, "scheduling points:") || strings.Contains(out, "scheduler hint at ") {
		t.Fatal(code, out, errText)
	}
}
