package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	analyseCleanFixture    = "../../testdata/tooling/analyse_cli/clean.sec"
	analyseAdvisoryFixture = "../../testdata/tooling/analyse_cli/advisory.sec"
	analyseRejectedFixture = "../../testdata/tooling/analyse_cli/rejected.sec"
)

// The default `sec analyse` presentation is silent on a clean result, shows
// only actionable findings otherwise, and --info adds one concise summary that
// --verbose includes without duplicating it (MD-017).
//
// Rules:
//   - rules/tooling/cli.md — § 5.4
//   - rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md — §§ 5, 11(1), 11(7)
func TestAnalysePresentationModes(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", analyseCleanFixture)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean analysis is not silent: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, code = runCLIForDiagnostics(t, "analyse", analyseAdvisoryFixture)
	if code != 0 || !strings.Contains(stdout, "advisory pitfall.bounds.skipped-zero at "+analyseAdvisoryFixture+":5:") || strings.Contains(stdout, "call graph:") || strings.Contains(stdout, "analysis summary:") {
		t.Fatalf("default output: %d\n%s", code, stdout)
	}
	allOutput, _, _ := runCLIForDiagnostics(t, "analyse", "--all", analyseAdvisoryFixture)
	if allOutput != stdout {
		t.Fatalf("--all differs from the default:\n%s\n---\n%s", allOutput, stdout)
	}
	info, _, _ := runCLIForDiagnostics(t, "analyse", "--info", analyseAdvisoryFixture)
	for _, want := range []string{"analysis summary:", "analyses: all (pitfalls, parameter-usage, stack, closures)", "not run alone: data-races", "scope: whole program", "depth: deep", "results: 0 errors, 0 unproven, 0 warnings, 2 advisory", "limitations: none"} {
		if !strings.Contains(info, want) {
			t.Errorf("--info lacks %q:\n%s", want, info)
		}
	}
	if strings.Contains(info, "call graph:") {
		t.Errorf("--info dumps facts:\n%s", info)
	}
	verbose, _, _ := runCLIForDiagnostics(t, "analyse", "--info", "--verbose", analyseAdvisoryFixture)
	if !strings.Contains(verbose, "call graph:") || strings.Count(verbose, "analysis summary:") != 1 || strings.Count(verbose, "results: ") != 1 {
		t.Fatalf("--verbose must include the details and one summary:\n%s", verbose)
	}
}

// -f and --function are the same analysis root: the selected callable and
// everything it reaches, with the call path to callee findings; ambiguous and
// unknown selectors are rejected without guessing (MD-017 § 3).
func TestAnalyseFunctionFocus(t *testing.T) {
	short, _, code := runCLIForDiagnostics(t, "analyse", "-f", "Helper", analyseAdvisoryFixture)
	long, _, _ := runCLIForDiagnostics(t, "analyse", "--function", "Helper", analyseAdvisoryFixture)
	if code != 0 || short != long {
		t.Fatalf("-f and --function differ: %d\n%s\n---\n%s", code, short, long)
	}
	if !strings.Contains(short, ":5:") || !strings.Contains(short, "via Helper -> Walk") || strings.Contains(short, ":15:") {
		t.Fatalf("focus must report the reachable Walk finding only:\n%s", short)
	}
	info, _, _ := runCLIForDiagnostics(t, "analyse", "--info", "-f", "Helper", analyseAdvisoryFixture)
	if !strings.Contains(info, "scope: Helper and 1 reachable callable(s); the whole program was analysed") {
		t.Fatalf("focus summary:\n%s", info)
	}
	_, stderr, code := runCLIForDiagnostics(t, "analyse", "-f", "Read", analyseAdvisoryFixture)
	if code != 1 || !strings.Contains(stderr, `"Read" names 2 callables`) || strings.Count(stderr, "declaration|") != 2 {
		t.Fatalf("ambiguous selector: %d %s", code, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "analyse", "--function", "Missing", analyseAdvisoryFixture)
	if code != 1 || !strings.Contains(stderr, `no callable named "Missing"`) {
		t.Fatalf("unknown selector: %d %s", code, stderr)
	}
}

// --depth and -d select the analysis budget; a reduced budget is reported as
// a limitation and never as additional proof (MD-017 § 4).
func TestAnalyseDepthSelection(t *testing.T) {
	short, _, code := runCLIForDiagnostics(t, "analyse", "--info", "-d", "standard", analyseAdvisoryFixture)
	long, _, _ := runCLIForDiagnostics(t, "analyse", "--info", "--depth", "standard", analyseAdvisoryFixture)
	if code != 0 || short != long || !strings.Contains(short, "depth: standard") || !strings.Contains(short, "limitation: depth standard") {
		t.Fatalf("depth selection: %d\n%s\n---\n%s", code, short, long)
	}
	deep, _, _ := runCLIForDiagnostics(t, "analyse", "--info", "--depth", "deep", analyseAdvisoryFixture)
	defaultDepth, _, _ := runCLIForDiagnostics(t, "analyse", "--info", analyseAdvisoryFixture)
	if deep != defaultDepth {
		t.Fatalf("--depth deep differs from the default")
	}
	_, stderr, code := runCLIForDiagnostics(t, "analyse", "-d", "exhaustive", analyseAdvisoryFixture)
	if code != 1 || !strings.Contains(stderr, "-d must be interactive, standard or deep") {
		t.Fatalf("invalid depth: %d %s", code, stderr)
	}
}

// Individual analyses are selected by their registered names; families that
// cannot run alone fail with an explanation instead of a clean result
// (MD-017 §§ 2, 7(4)).
func TestAnalyseFamilySelection(t *testing.T) {
	pitfalls, _, code := runCLIForDiagnostics(t, "analyse", "pitfalls", analyseAdvisoryFixture)
	if code != 0 || !strings.Contains(pitfalls, "pitfall.bounds.skipped-zero") {
		t.Fatalf("pitfalls: %d %s", code, pitfalls)
	}
	usage, _, code := runCLIForDiagnostics(t, "analyse", "parameter-usage", analyseAdvisoryFixture)
	if code != 0 || strings.Contains(usage, "pitfall.") {
		t.Fatalf("parameter-usage must not report pitfalls: %d %s", code, usage)
	}
	for _, name := range []string{"isr", "deadlocks", "data-races"} {
		stdout, stderr, code := runCLIForDiagnostics(t, "analyse", name, analyseAdvisoryFixture)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "the "+name+" analysis cannot run on its own") {
			t.Fatalf("%s: %d %q %s", name, code, stdout, stderr)
		}
	}
}

// --json writes one versioned sec.analysis-report on stdout, also with no
// findings, with a focus and for a rejected program (MD-017 § 6).
func TestAnalyseJSONReport(t *testing.T) {
	decode := func(text string) analysisReportDocument {
		t.Helper()
		var document analysisReportDocument
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&document); err != nil || decoder.More() {
			t.Fatalf("not one analysis report: %v\n%s", err, text)
		}
		if document.Format != analysisReportFormat || document.Version != analysisReportFormatVersion {
			t.Fatalf("header %s v%d", document.Format, document.Version)
		}
		return document
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", analyseCleanFixture)
	clean := decode(stdout)
	if code != 0 || stderr != "" || clean.Status != "complete" || len(clean.Findings) != 0 || clean.Focus != nil || clean.Depth != "deep" {
		t.Fatalf("clean report: %d %s %+v", code, stderr, clean)
	}
	stdout, _, code = runCLIForDiagnostics(t, "analyse", "--json", "-f", "Helper", analyseAdvisoryFixture)
	focused := decode(stdout)
	if code != 0 || focused.Focus == nil || focused.Focus.Name != "Helper" || len(focused.Focus.Reachable) != 2 || len(focused.Findings) != 1 ||
		focused.Findings[0].Path != "Helper -> Walk" || focused.Summary.Advisory != 1 {
		t.Fatalf("focused report: %d %+v", code, focused)
	}
	again, _, _ := runCLIForDiagnostics(t, "analyse", "--json", "-f", "Helper", analyseAdvisoryFixture)
	if again != stdout {
		t.Fatal("JSON report is not deterministic")
	}
	stdout, _, code = runCLIForDiagnostics(t, "analyse", "--json", analyseRejectedFixture)
	rejected := decode(stdout)
	if code != 3 || rejected.Status != "rejected" || rejected.Summary.Errors != 1 || len(rejected.Diagnostics) != 1 {
		t.Fatalf("rejected report: %d %+v", code, rejected)
	}
	_, stderr, code = runCLIForDiagnostics(t, "analyse", "--json", "--info", analyseCleanFixture)
	if code != 1 || !strings.Contains(stderr, "--json is a separate output format") {
		t.Fatalf("--json with --info: %d %s", code, stderr)
	}
}

// `sec help`, `sec --help`, `sec help <command>` and `sec <command> --help`
// render from the one command registry; unknown names get mentor-style
// suggestions (MD-017 § 8).
func TestCLIHelpForms(t *testing.T) {
	top, _, code := runCLIForDiagnostics(t, "help")
	dashed, _, _ := runCLIForDiagnostics(t, "--help")
	if code != 0 || top != dashed || !strings.Contains(top, "analyse") || !strings.Contains(top, "sec help <command>") {
		t.Fatalf("top-level help: %d\n%s\n---\n%s", code, top, dashed)
	}
	topic, _, code := runCLIForDiagnostics(t, "help", "analyse")
	contextual, _, _ := runCLIForDiagnostics(t, "analyse", "--help")
	if code != 0 || topic != contextual {
		t.Fatalf("command help differs: %d\n%s\n---\n%s", code, topic, contextual)
	}
	for _, family := range analysisFamilies() {
		if !strings.Contains(topic, "  "+family.Name+" ") {
			t.Errorf("analyse help does not list %s", family.Name)
		}
	}
	for _, want := range []string{"-f, --function", "-d, --depth", "--info", "--verbose", "--json", "sec analyse -f myFunc", "sec analyse deadlocks --function myFunc", "not the INFO diagnostic severity"} {
		if !strings.Contains(topic, want) {
			t.Errorf("analyse help lacks %q", want)
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "anlyse", analyseCleanFixture)
	if code != 1 || !strings.Contains(stderr, `unknown command "anlyse" (did you mean "analyse"?)`) || !strings.Contains(stderr, "sec help") {
		t.Fatalf("unknown command: %d %s", code, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "help", "analize")
	if code != 1 || !strings.Contains(stderr, `did you mean "analyse"`) {
		t.Fatalf("unknown help topic: %d %s", code, stderr)
	}
}
