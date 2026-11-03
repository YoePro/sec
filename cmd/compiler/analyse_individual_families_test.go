package main

import (
	"encoding/json"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Individually selected analyses isolate optional findings from a shared mixed
// source while retaining the registered diagnostic owner, source and severity.
// This completes the owning analysis books' explicit-selector acceptance, not
// their broader semantic or code-action implementation requirements.
// Rules: rules/tooling/cli.md — §§5.1–5.6,6;
// rules/analysis/pitfall_analysis.md — "sec analyse";
// rules/analysis/parameter_usage_analysis.md — "sec analyse";
// rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — §§2–6,11.
func TestAnalyseIndividualFamilies(t *testing.T) {
	const fixture = "../../testdata/tooling/analyse_cli/mixed_families.sec"
	decode := func(args ...string) analysisReportDocument {
		t.Helper()
		out, stderr, code := runCLIForDiagnostics(t, append([]string{"analyse"}, args...)...)
		var doc analysisReportDocument
		if err := json.Unmarshal([]byte(out), &doc); err != nil || code != 0 || stderr != "" {
			t.Fatalf("%v: %d %v %s %s", args, code, err, out, stderr)
		}
		if doc.Format != analysisReportFormat || doc.Version != analysisReportFormatVersion {
			t.Fatal(doc)
		}
		return doc
	}
	all := decode("--json", fixture)
	if len(all.Findings) != 1 || len(all.Diagnostics) != 2 {
		t.Fatalf("fixture must exercise both analysis owners: %+v", all)
	}
	for _, family := range []string{"pitfalls", "parameter-usage"} {
		t.Run(family, func(t *testing.T) {
			doc := decode(family, "--json", fixture)
			if doc.Status != "complete" || len(doc.Analyses) != 1 || doc.Analyses[0].Name != family || doc.Analyses[0].Status != "executed" || !doc.Analyses[0].Complete {
				t.Fatal(doc)
			}
			wantFindings, wantDiagnostics := 1, 0
			if family == "parameter-usage" {
				wantFindings, wantDiagnostics = 0, 2
			}
			if len(doc.Findings) != wantFindings || len(doc.Diagnostics) != wantDiagnostics || doc.Summary.Advisory != wantFindings+wantDiagnostics {
				t.Fatal(doc)
			}
			for _, finding := range doc.Findings {
				if finding.Analysis != family || finding.Rule != "pitfall.bounds.skipped-zero" || finding.Location.Start.Line != 13 {
					t.Fatal(finding)
				}
			}
			for _, diagnostic := range doc.Diagnostics {
				if diagnostic.ID == nil || *diagnostic.ID != diagnostics.LargeValueParameter || diagnostic.Severity != diagnostics.SeverityInformation {
					t.Fatal(diagnostic)
				}
			}
			text, stderr, code := runCLIForDiagnostics(t, "analyse", family, fixture)
			if code != 0 {
				t.Fatal(code, text, stderr)
			}
			text += stderr
			if family == "pitfalls" && (!strings.Contains(text, "pitfall.bounds.skipped-zero") || strings.Contains(text, "passes large array")) {
				t.Fatal(text)
			}
			if family == "parameter-usage" && (!strings.Contains(text, "passes large array") || strings.Contains(text, "pitfall.")) {
				t.Fatal(text)
			}
			verbose, stderr, code := runCLIForDiagnostics(t, "analyse", family, "--verbose", fixture)
			expected, excluded := "pitfalls:", "parameter usage:"
			if family == "parameter-usage" {
				expected, excluded = excluded, expected
			}
			if code != 0 || !strings.Contains(verbose, expected) || strings.Contains(verbose, excluded) || strings.Contains(verbose, "call graph:") {
				t.Fatal(code, verbose, stderr)
			}
			root := "Root"
			if family == "parameter-usage" {
				root = "Recommend"
			}
			focused := decode(family, "--json", "-f", root, fixture)
			if focused.Focus == nil || focused.Focus.Name != root || focused.Summary.Advisory != 1 {
				t.Fatal(focused)
			}
			if family == "pitfalls" && (len(focused.Findings) != 1 || focused.Findings[0].Path != "Root -> Walk") {
				t.Fatal(focused)
			}
			if family == "parameter-usage" && (len(focused.Diagnostics) != 1 || focused.Diagnostics[0].Primary == nil || focused.Diagnostics[0].Primary.Span.Start.Line != 3) {
				t.Fatal(focused)
			}
			clean, stderr, code := runCLIForDiagnostics(t, "analyse", family, analyseCleanFixture)
			if code != 0 || clean != "" || stderr != "" {
				t.Fatal(code, clean, stderr)
			}
			rejected, stderr, code := runCLIForDiagnostics(t, "analyse", family, "--json", analyseRejectedFixture)
			var invalid analysisReportDocument
			if err := json.Unmarshal([]byte(rejected), &invalid); err != nil || code != 3 || invalid.Status != "rejected" || len(invalid.Diagnostics) == 0 {
				t.Fatalf("mandatory diagnostic lost: %d %v %s %s", code, err, rejected, stderr)
			}
		})
	}
}

// Names are canonical registry entries: no legacy alias or --all combination
// may silently select another analysis. Parsing failure is distinct from a
// supported family with no optional findings.
// Rules: rules/tooling/cli.md — §§5.1–5.2; MD-017 — §§2,11(3).
func TestAnalyseIndividualFamilyNames(t *testing.T) {
	for _, alias := range []string{"pitfall", "parameters", "parameter-usage-analysis"} {
		if _, err := parseAnalyseCommandArgs([]string{alias}, hostCompilerTarget()); err == nil {
			t.Fatal("unregistered alias accepted", alias)
		}
	}
	for _, family := range []string{"pitfalls", "parameter-usage"} {
		if _, err := parseAnalyseCommandArgs([]string{"--all", family}, hostCompilerTarget()); err == nil {
			t.Fatal("conflicting selection accepted", family)
		}
		cmd, err := parseAnalyseCommandArgs([]string{family, "--depth", "standard", "--json"}, hostCompilerTarget())
		if err != nil || cmd.family != family || cmd.presentation != analysePresentJSON {
			t.Fatal(cmd, err)
		}
	}
}
