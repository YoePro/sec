package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// analyseAgreementSnapshot runs fresh mandatory producers, optionally reuses
// proven-compatible pitfall body results, and feeds their facts to the same
// family/focus report builder used by sec analyse. The fixture has no imports
// or external contracts: hashing its complete source conservatively invalidates
// every prerequisite on any edit; target, depth and model separate cache scopes.
// This is a controlled cache host, not a CLI workspace/cache implementation.
// Rules: rules/tooling/cli.md §§5–7; rules/compiler/incremental_compilation.md
// §§1–9; rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md §11(11).
func analyseAgreementSnapshot(t *testing.T, file, source string, target CompilerTarget, depth sema.AnalysisDepth, cache *sema.PitfallCache) (map[string]analyseReport, sema.PitfallCacheUsage) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	definition, ok := findTargetDefinition(target)
	if !ok {
		t.Fatal(target)
	}
	plan, err := definition.scalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	a := sema.NewAnalyzerWithScalarPlanAndDepth(plan, depth)
	var usage sema.PitfallCacheUsage
	var semanticErrors []sema.Error
	if cache == nil {
		semanticErrors = a.Analyze(program)
	} else {
		fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
		var revisions []sema.PitfallBodyRevision
		for _, statement := range program.Statements {
			fn, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			revision := sema.PitfallBodyRevision{Body: sema.PitfallBodyIdentity(fn.Body), Complete: true}
			for _, kind := range sema.PitfallRequiredFactKinds(depth) {
				revision.Dependencies = append(revision.Dependencies, sema.PitfallDependency{Kind: kind, Identity: file, Fingerprint: fingerprint})
			}
			revisions = append(revisions, revision)
		}
		scope := sema.PitfallCacheScope{Module: "main", CompilationPlan: target.String(), CompilerModel: "analyse-agreement-v1"}
		semanticErrors, usage, err = a.AnalyzeWithPitfallCache(program, cache, scope, revisions)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(semanticErrors) != 0 {
		t.Fatal(semanticErrors)
	}
	selected := map[string]bool{file: true}
	reports := map[string]analyseReport{}
	for _, family := range []string{"", "pitfalls", "parameter-usage", "stack", "closures"} {
		for _, focus := range []string{"", "Root", "Recommend"} {
			var scope *analyseScope
			if focus != "" {
				scope, err = resolveAnalyseScope(a.CallGraph(), focus, selected)
				if err != nil {
					t.Fatal(err)
				}
			}
			reports[family+"/"+focus] = buildAnalyseReport(a, selected, target, family, scope)
		}
	}
	// Focused findings must also equal the applicable subset of the whole-unit
	// facts, with only the canonical callable owner added by focus presentation.
	for _, focused := range reports {
		if focused.scope == nil {
			continue
		}
		whole := reports[focused.family+"/"]
		var findings []analyseFinding
		for _, finding := range whole.findings {
			if focused.scope.includes(finding.Token) {
				finding.Callable, _ = focused.scope.owner(finding.Token)
				findings = append(findings, finding)
			}
		}
		var warnings []sema.Error
		for _, warning := range whole.warnings {
			if focused.scope.includes(lexer.Token{File: warning.File, Line: warning.Line, Column: warning.Column}) {
				warnings = append(warnings, warning)
			}
		}
		if !reflect.DeepEqual(findings, focused.findings) || !reflect.DeepEqual(warnings, focused.warnings) {
			t.Fatal("focused canonical findings disagree with whole-unit findings")
		}
	}
	return reports, usage
}

// Actual cache hits, post-edit invalidation and concurrent shared-cache readers
// must preserve the entire canonical human/focus report (warnings, evidence,
// severity, counts, limitations and paths) for each supported family and depth.
// Rules: rules/tooling/cli.md §7(2); MD-017 correction §11(11);
// rules/compiler/incremental_compilation.md §§1–9,29–34.
func TestAnalyseColdIncrementalParallelAgreement(t *testing.T) {
	file := "../../testdata/tooling/analyse_cli/mixed_families.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, targetName := range []string{"linux-amd64", "linux-armv7"} {
		target, _ := parseCompilerTarget(targetName)
		for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
			t.Run(targetName+"/"+string(depth), func(t *testing.T) {
				cold, _ := analyseAgreementSnapshot(t, file, source, target, depth, nil)
				var cache sema.PitfallCache
				first, _ := analyseAgreementSnapshot(t, file, source, target, depth, &cache)
				warm, usage := analyseAgreementSnapshot(t, file, source, target, depth, &cache)
				if !reflect.DeepEqual(cold, first) || !reflect.DeepEqual(cold, warm) {
					t.Fatal("cold/warm reports disagree")
				}
				if depth != sema.AnalysisInteractive && usage.Hits == 0 {
					t.Fatal("no actual incremental reuse", usage)
				}
				t.Run("parallel", func(t *testing.T) {
					for worker := 0; worker < 4; worker++ {
						t.Run(fmt.Sprint(worker), func(t *testing.T) {
							t.Parallel()
							parallel, _ := analyseAgreementSnapshot(t, file, source, target, depth, &cache)
							if !reflect.DeepEqual(cold, parallel) {
								t.Fatal("parallel report differs from cold report")
							}
						})
					}
				})
				edited := strings.Replace(source, "uint(1)..<values.Len", "uint(0)..<values.Len", 1)
				if edited == source {
					t.Fatal("missing edit anchor")
				}
				editedCold, _ := analyseAgreementSnapshot(t, file, edited, target, depth, nil)
				editedWarm, editedUsage := analyseAgreementSnapshot(t, file, edited, target, depth, &cache)
				if !reflect.DeepEqual(editedCold, editedWarm) {
					t.Fatal("edited report kept stale findings")
				}
				if depth != sema.AnalysisInteractive && editedUsage.Misses == 0 {
					t.Fatal("edit did not invalidate reuse", editedUsage)
				}
				if depth == sema.AnalysisDeep && reflect.DeepEqual(cold["pitfalls/"].findings, editedWarm["pitfalls/"].findings) {
					t.Fatal("edit did not change the expected finding")
				}
				restored, _ := analyseAgreementSnapshot(t, file, source, target, depth, &cache)
				if !reflect.DeepEqual(cold, restored) {
					t.Fatal("edit history changed restored report")
				}
			})
		}
	}
}

// Built compiler executables must agree with the ordinary CLI harness on exact
// stdout/stderr and exit status, with and without Go optimizations. Builds stay
// in a temporary directory and use trimpath; the matrix exercises help, failure,
// incomplete proof, family/focus selection, human/JSON and target changes.
// Rules: rules/tooling/cli.md §7; MD-017 correction §11(9–11).
func TestAnalyseOptimizedBuildAgreement(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile("../../testdata/tooling/analyse_cli/mixed_families.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.sec")
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"help"}, {"help", "analyse"}, {"--help"}, {"analyse", "--help"},
		{"analyse", "pitfalls", "--json", "--depth", "deep", file},
		{"analyse", "parameter-usage", "--json", "-f", "Recommend", file},
		{"analyse", "pitfalls", "--json", "-f", "Root", "--target", "linux-armv7", file},
		{"analyse", "--verbose", file},
		{"analyse", "pitfalls", "--json", "--depth", "interactive", file},
		{"analyse", "isr", file}, {"analyse", "pitfall", file},
		{"analyse", "--json", analyseRejectedFixture},
		{"analyse", "pitfalls", "--json", analyseCleanFixture},
	}
	type result struct {
		stdout, stderr string
		code           int
	}
	baseline := make([]result, len(cases))
	for i, args := range cases {
		baseline[i].stdout, baseline[i].stderr, baseline[i].code = runCLIForDiagnostics(t, args...)
		wantCode := 0
		if args[0] == "analyse" && (args[1] == "isr" || args[1] == "pitfall") {
			wantCode = 1
		}
		if args[len(args)-1] == analyseRejectedFixture {
			wantCode = 3
		}
		if baseline[i].code != wantCode {
			t.Fatalf("invalid baseline for %v: %+v", args, baseline[i])
		}
		if strings.Contains(strings.Join(args, " "), "--json") {
			var document analysisReportDocument
			if err := json.Unmarshal([]byte(baseline[i].stdout), &document); err != nil || document.Format != analysisReportFormat || document.Version != analysisReportFormatVersion {
				t.Fatalf("invalid baseline report for %v: %v %+v", args, err, baseline[i])
			}
			status := "complete"
			if wantCode == 3 {
				status = "rejected"
			} else if document.Depth == "interactive" {
				status = "incomplete"
			}
			if document.Status != status {
				t.Fatalf("baseline hid proof state for %v: %+v", args, document)
			}
		}
	}
	for _, mode := range []string{"optimized", "unoptimized"} {
		t.Run(mode, func(t *testing.T) {
			binary := filepath.Join(root, mode+"-sec")
			args := []string{"build", "-trimpath", "-p", "4", "-o", binary}
			if mode == "unoptimized" {
				args = append(args, "-gcflags=all=-N -l")
			}
			args = append(args, ".")
			build := exec.Command("go", args...)
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, output)
			}
			for i, args := range cases {
				command := exec.Command(binary, args...)
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				code := 0
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else if err != nil {
					t.Fatal(err)
				}
				got := result{stdout.String(), stderr.String(), code}
				if got != baseline[i] {
					t.Fatalf("%v differs:\ngot: %+v\nwant: %+v", args, got, baseline[i])
				}
			}
		})
	}
}
