package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAnalyseReportUsesCanonicalSnapshot checks actual Sema depth, advisory
// severity, source-qualified selection, determinism and unchanged owned facts.
// Rules: rules/compiler/compiler.md — §56(3–4);
// rules/compiler/compiler_analysis.md — §61(2–3), §62(1–3).
func TestAnalyseReportUsesCanonicalSnapshot(t *testing.T) {
	file := "../../testdata/sema/parameter_recommendations_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.Parse()
		if program.HasErrors {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithDepth(depth)
		if errors := a.Analyze(program.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		warnings, allocations := a.Warnings(), a.AllocationFacts()
		info, warning := 0, 0
		for _, value := range warnings {
			if value.Severity == diagnostics.SeverityInformation {
				info++
			} else {
				warning++
			}
		}
		if info == 0 {
			t.Fatal("fixture did not exercise informational diagnostics")
		}
		selected := map[string]bool{file: true}
		report := buildAnalyseReport(a, selected, hostCompilerTarget(), "", nil)
		// The forwarding call also exercises independent final-use move
		// advice. Count canonical optional findings alongside owner warnings.
		pitfallInfo := 0
		for _, finding := range a.PitfallAnalysis().Findings() {
			if selected[finding.Subject.Source.File] && finding.DiagnosticID == "" && finding.Classification != sema.PitfallProvenInvalid {
				pitfallInfo++
			}
		}
		if pitfallInfo == 0 {
			t.Fatal("fixture did not exercise independent pitfall advice")
		}
		if report.depth != depth || report.counts[analyseClassAdvisory] != info+pitfallInfo || report.counts[analyseClassWarning] != warning {
			t.Fatal(depth, warnings, report)
		}
		var output bytes.Buffer
		report.write(&output)
		if !strings.Contains(output.String(), "analysis report (depth "+string(depth)+",") {
			t.Fatal(output.String())
		}
		if !reflect.DeepEqual(report, buildAnalyseReport(a, selected, hostCompilerTarget(), "", nil)) || !reflect.DeepEqual(warnings, a.Warnings()) || !reflect.DeepEqual(allocations, a.AllocationFacts()) {
			t.Fatal("report changed or recomputed owned facts")
		}
		other := buildAnalyseReport(a, map[string]bool{"another.sec": true}, hostCompilerTarget(), "", nil)
		if other.counts[analyseClassAdvisory] != 0 || other.counts[analyseClassWarning] != 0 {
			t.Fatal("unselected diagnostics counted", other.counts)
		}
	}
}

// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §61(3);
// rules/tooling/diagnostics.md — §4(6–9).
func TestAnalyseDiagnosticClassesUseOwnerFacts(t *testing.T) {
	for _, test := range []struct {
		value sema.Error
		want  analyseReportClass
	}{
		{sema.Error{Severity: diagnostics.SeverityInformation, ProofState: diagnostics.ProofInvalid}, analyseClassAdvisory},
		{sema.Error{Severity: diagnostics.SeverityWarning, ProofState: diagnostics.ProofUnproven}, analyseClassWarning},
		{sema.Error{Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofUnproven}, analyseClassUnproven},
		{sema.Error{Severity: diagnostics.SeverityError, Message: "Unproven"}, analyseClassError},
		{sema.Error{ID: diagnostics.LargeValueParameter}, analyseClassAdvisory},
		{sema.Error{ID: diagnostics.RequiredProofUnavailable, Severity: diagnostics.SeverityInformation, ProofState: diagnostics.ProofUnproven}, analyseClassUnproven},
	} {
		if got := analyseDiagnosticClass(test.value); got != test.want {
			t.Fatal(test.value, got, test.want)
		}
	}
}

// TestAnalyseCLIAdvisoryCountsMatchOccurrences guards the public report against
// counting the same informational owner as a warning or a second diagnostic.
// Rules: rules/compiler/compiler_analysis.md — §61(3).
func TestAnalyseCLIAdvisoryCountsMatchOccurrences(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/sema/parameter_recommendations_valid.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	doc := decodeOccurrenceDocument(t, stderr)
	if doc.Summary.Information == 0 || doc.Summary.Warnings != 0 || !strings.Contains(stdout, "0 warnings, ") {
		t.Fatal(stdout, doc)
	}
	// Owning diagnostics and optional findings have distinct transports. This
	// fixture adds one final-use move finding to its informational occurrences.
	if strings.Count(stdout, "advisory pitfall.ownership.last-use-argument-copy at ") != 1 || !strings.Contains(stdout, fmt.Sprintf("%d advisory", doc.Summary.Information+1)) {
		t.Fatal("report disagrees with occurrence owners", stdout, doc)
	}
}
