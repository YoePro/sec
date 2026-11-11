package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Explicit ISR selection inspects canonical prerequisites but cannot establish
// ISR safety without resolved profiles and their mandatory proof producers.
// Human and JSON output must expose incompleteness at every depth, even when
// no potential panic/allocation effect is recorded; source rejection stays mandatory.
// Rules: rules/analysis/isr_analysis.md §§1–2,32–33;
// rules/tooling/cli.md §§5.3–5.6,6.
func TestAnalyseISRPrerequisiteInspection(t *testing.T) {
	const fixture = "../../testdata/tooling/analyse_cli/isr_prerequisites.sec"
	for _, depth := range []string{"interactive", "standard", "deep"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "isr", "--json", "-f", "Root", "-d", depth, fixture)
		var report analysisReportDocument
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err, stdout)
		}
		if code != 1 || stderr != "" || report.Status != "incomplete" || report.Focus == nil || report.Focus.Name != "Root" || len(report.Focus.Reachable) != 2 || len(report.Analyses) != 1 || report.Analyses[0].Name != "isr" || report.Analyses[0].Status != "executed" || report.Analyses[0].Complete || report.Analyses[0].Note != isrInspectionLimitation || len(report.Findings) != 0 || len(report.Diagnostics) != 0 || !strings.Contains(strings.Join(report.Limitations, "\n"), isrInspectionLimitation) {
			t.Fatalf("%s: exit %d stderr %s report %+v", depth, code, stderr, report)
		}
		again, _, _ := runCLIForDiagnostics(t, "analyse", "isr", "--json", "--function", "Root", "--depth", depth, fixture)
		if again != stdout {
			t.Fatal("ISR report or selector aliases are nondeterministic")
		}
	}
	for _, focus := range []string{"Root", "NoEffects", "Opaque", "AllocatingRoot"} {
		output, stderr, code := runCLIForDiagnostics(t, "analyse", "isr", "--verbose", "-f", focus, fixture)
		if code != 1 || stderr != "" || !strings.Contains(output, "ISR prerequisites (not a safety proof)") || !strings.Contains(output, "ISR analysis is incomplete") || strings.Contains(output, "Unrelated:") || strings.Contains(output, "pitfalls:") || strings.Contains(output, "closures:") {
			t.Fatal(focus, code, output, stderr)
		}
		if focus == "Root" && (!strings.Contains(output, "Root: may panic via Root -> Leaf") || !strings.Contains(output, "ISR prerequisite witness") || strings.Contains(output, "unrelated witness")) {
			t.Fatal("canonical panic path/cause missing or unrelated cause leaked", output)
		}
		if focus == "NoEffects" && !strings.Contains(output, "may-panic=false, may-allocate=false") {
			t.Fatal(output)
		}
		if focus == "Opaque" && !strings.Contains(output, "may-panic-unknown-callee") {
			t.Fatal(output)
		}
		if focus == "Opaque" && !strings.Contains(output, "allocation-unknown=true") {
			t.Fatal("unknown allocation was presented as absent", output)
		}
		if focus == "AllocatingRoot" && !strings.Contains(output, "AllocatingRoot: may allocate via AllocatingRoot -> AllocatingLeaf") {
			t.Fatal("canonical allocation path missing", output)
		}
	}
	output, stderr, code := runCLIForDiagnostics(t, "analyse", "isr", analyseCleanFixture)
	if code != 1 || stderr != "" || !strings.Contains(output, "incomplete: ISR analysis is incomplete") || strings.Contains(output, "callable summary") {
		t.Fatal(code, output, stderr)
	}
	rejected, stderr, code := runCLIForDiagnostics(t, "analyse", "isr", "--json", analyseRejectedFixture)
	var report analysisReportDocument
	if err := json.Unmarshal([]byte(rejected), &report); err != nil || code != 3 || report.Status != "rejected" || len(report.Diagnostics) == 0 {
		t.Fatal(err, code, rejected, stderr)
	}
}
