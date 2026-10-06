package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Dependencies and identity/priority tie breaks determine the schedule, never
// callback registration order. Validate all producers before running any work.
// Rules: rules/compiler/compiler_analysis.md — §9(1–4), §10(4–5), §14(2).
func TestAnalysisScheduleDependencyOrderIsDeterministic(t *testing.T) {
	var called []string
	pass := func(id string, priority int, dependencies ...string) analysisPass {
		return analysisPass{id: id, priority: priority, dependencies: dependencies, run: func() analysisStepResult {
			called = append(called, id)
			return analysisStepResult{Iterations: 1, Converged: true}
		}}
	}
	passes := []analysisPass{
		pass("consumer", -1, "left", "right"),
		pass("independent", 10),
		pass("right", 5, "seed"),
		pass("left", 5, "seed"),
		pass("last", 0, "consumer"),
	}
	want := []string{"left", "right", "consumer", "last", "independent"}
	var original []AnalysisPassRecord
	var visit func(int)
	visit = func(index int) {
		if index == len(passes) {
			called = nil
			records, err := runAnalysisSchedule(passes, []string{"seed"})
			if err != nil || !reflect.DeepEqual(called, want) {
				t.Fatalf("schedule = %v, %v; want %v", called, err, want)
			}
			if original == nil {
				original = records
			}
			if !reflect.DeepEqual(records, original) {
				t.Fatal("registration changed provenance")
			}
			return
		}
		for next := index; next < len(passes); next++ {
			passes[index], passes[next] = passes[next], passes[index]
			visit(index + 1)
			passes[index], passes[next] = passes[next], passes[index]
		}
	}
	visit(0)
}

func TestAnalysisScheduleRejectsInvalidGraphBeforeExecution(t *testing.T) {
	called := false
	run := func() analysisStepResult { called = true; return analysisStepResult{Converged: true} }
	for _, test := range []struct {
		name    string
		passes  []analysisPass
		facts   []string
		message string
	}{
		{"missing producer", []analysisPass{{id: "independent", run: run}, {id: "needs-fact", dependencies: []string{"absent"}, run: run}}, nil, "missing producer"},
		{"duplicate", []analysisPass{{id: "same", run: run}, {id: "same", run: run}}, nil, "duplicate analysis producer"},
		{"fact collision", []analysisPass{{id: "seed", run: run}}, []string{"seed"}, "duplicate analysis producer"},
		{"duplicate initial fact", nil, []string{"seed", "seed"}, "duplicate initial"},
		{"empty initial fact", nil, []string{""}, "initial analysis fact"},
		{"nil callback", []analysisPass{{id: "invalid"}}, nil, "invalid analysis pass"},
		{"empty identity", []analysisPass{{run: run}}, nil, "invalid analysis pass"},
		{"duplicate dependency", []analysisPass{{id: "consumer", dependencies: []string{"seed", "seed"}, run: run}}, []string{"seed"}, "repeats dependency"},
		{"self cycle", []analysisPass{{id: "self", dependencies: []string{"self"}, run: run}}, nil, "coordinated fixed point: self"},
		{"mutual cycle", []analysisPass{{id: "b", dependencies: []string{"a"}, run: run}, {id: "a", dependencies: []string{"b"}, run: run}, {id: "independent", run: run}}, nil, "coordinated fixed point: a, b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			called = false
			_, err := runAnalysisSchedule(test.passes, test.facts)
			if err == nil || !strings.Contains(err.Error(), test.message) || called {
				t.Fatalf("invalid plan ran=%t, err=%v", called, err)
			}
		})
	}
}

// Coupled cyclic facts are solved within one bounded pass; incomplete
// producers must widen before any consumer runs.
// Rules: rules/compiler/compiler_analysis.md — §9(4), §14(1–3), §15(3–5).
func TestAnalysisFixedPointConvergenceAndConservativePublication(t *testing.T) {
	for _, limit := range []int{1, 8} {
		x, y := 0, 0
		widened, consumed := false, false
		passes := []analysisPass{
			{id: "consumer", dependencies: []string{"cyclic-facts"}, run: func() analysisStepResult {
				consumed = true
				if !widened && (x != 3 || y != 3) {
					t.Fatal("consumer saw intermediate facts")
				}
				return analysisStepResult{Converged: true}
			}},
			{id: "cyclic-facts", run: func() analysisStepResult {
				return runAnalysisFixedPoint(limit, func() bool {
					beforeX, beforeY := x, y
					y = x
					if x < 3 {
						x++
					}
					return beforeX != x || beforeY != y
				}, func() { widened = true })
			}},
		}
		records, err := runAnalysisSchedule(passes, nil)
		if err != nil || !consumed {
			t.Fatalf("schedule err=%v consumed=%t", err, consumed)
		}
		if limit == 1 && (records[0].Converged || !records[0].Widened || records[0].Iterations != 1) {
			t.Fatalf("exhausted solver = %+v", records[0])
		}
		if limit == 8 && (!records[0].Converged || records[0].Widened || records[0].Iterations != 5) {
			t.Fatalf("converged solver = %+v", records[0])
		}
	}
	consumerRan := false
	_, err := runAnalysisSchedule([]analysisPass{
		{id: "unfinished", run: func() analysisStepResult { return analysisStepResult{Iterations: 1} }},
		{id: "consumer", dependencies: []string{"unfinished"}, run: func() analysisStepResult { consumerRan = true; return analysisStepResult{Converged: true} }},
	}, nil)
	if err == nil || consumerRan {
		t.Fatalf("unfinished facts consumed=%t err=%v", consumerRan, err)
	}
	widenCalls := 0
	result := runAnalysisFixedPoint(1, func() bool { return false }, func() { widenCalls++ })
	if !result.Converged || result.Iterations != 1 || widenCalls != 0 {
		t.Fatalf("immediate convergence = %+v, widen=%d", result, widenCalls)
	}
}

// Compiler/LSP analyzers execute the same producer graph at every depth;
// recursion precision and conservative budget exhaustion remain explicit.
// Rules: rules/compiler/compiler_analysis.md — §§9–10, §14, §60(1–3).
func TestAnalyzerPublishesDependencyAndFixedPointSchedule(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/analysis_schedule_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			program := parser.New(lexer.New(string(source))).ParseProgram()
			analyzer := NewAnalyzerWithDepth(depth)
			if errors := analyzer.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			records := analyzer.AnalysisSchedule()
			seen := map[string]bool{"declaration-facts": true}
			for _, record := range records {
				for _, dependency := range record.Dependencies {
					if !seen[dependency] {
						t.Fatalf("%s ran before %s", record.ID, dependency)
					}
				}
				if !record.Converged && !record.Widened {
					t.Fatalf("unpublished producer: %+v", record)
				}
				seen[record.ID] = true
			}
			if len(records) != 10 || !seen["pitfalls"] {
				t.Fatalf("missing pipeline stages: %+v", records)
			}
			before := analyzer.AnalysisSchedule()
			records[0].ID = "mutated"
			records[0].Dependencies[0] = "mutated"
			if !reflect.DeepEqual(before, analyzer.AnalysisSchedule()) {
				t.Fatal("schedule is mutable")
			}
			// The mutual call cycle retains the demand derived by its leaf.
			for _, name := range []string{"Alpha", "Beta"} {
				demand := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), name).Parameters[0].Demand
				if demand.MinimumExtent != 7 || demand.Precision != ParameterDemandExact {
					t.Fatalf("%s demand: %+v", name, demand)
				}
			}
			analyzer.Analyze(program)
			if !reflect.DeepEqual(before, analyzer.AnalysisSchedule()) {
				t.Fatal("schedule changed on repeated analysis")
			}
			analyzer.analysisBudget.MaxSummaryIterations = 1
			analyzer.Analyze(program)
			widenedDemand := false
			for _, record := range analyzer.AnalysisSchedule() {
				if record.ID == "parameter-demand" {
					widenedDemand = record.Widened && !record.Converged && record.Iterations == 1
				}
			}
			widenedReferences := false
			for _, record := range analyzer.AnalysisSchedule() {
				if record.ID == "reference-summaries" {
					widenedReferences = record.Widened && !record.Converged && record.Iterations == 1
				}
			}
			if !widenedReferences || !analyzer.functions["Forward"][0].ReturnOrigin.Unknown {
				t.Fatal("reference budget exhaustion was not widened before consumers")
			}
			if !widenedDemand {
				t.Fatal("budget exhaustion lost conservative provenance")
			}
			if _, converged := analyzer.ParameterUsageAnalysis().InterproceduralStatus(); converged {
				t.Fatal("exhaustion claimed convergence")
			}
		})
	}
}

// Execution completion cannot certify source validity. Existing normative
// diagnostics remain errors even when all scheduled producers completed.
// Rule: rules/compiler/compiler_analysis.md — §10(5), §§7–8 proof ownership.
func TestAnalysisScheduleDoesNotTreatCompletionAsValidity(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/analysis_schedule_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer := NewAnalyzerWithDepth(depth)
		program := parser.New(lexer.New(string(source))).ParseProgram()
		if errors := analyzer.Analyze(program); len(errors) == 0 {
			t.Fatal("completed schedule erased normative type error")
		}
		if len(analyzer.AnalysisSchedule()) != 10 {
			t.Fatal("semantic recovery omitted execution provenance")
		}
	}
}
