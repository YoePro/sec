package sema

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
)

// Rules: rules/compiler/compiler_pipeline.md — §§23(3–4), 33(1–3), 34(2–3).
func TestIteratorLoweringReadinessRequiresCanonicalFacts(t *testing.T) {
	program := parameterRegressionProgram(t, "iterator_lowering_readiness_valid")
	analyzer := NewAnalyzerWithDepth(AnalysisInteractive)
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(analyzer.Warnings()) == 0 {
		t.Fatal("fixture did not exercise non-blocking advisory analysis")
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
	var loop *ast.ForStatement
	for stmt, requirement := range analyzer.iteratorLoweringRequirements {
		if requirement.required && requirement.caller == callableIDForName(analyzer, "Reusable") {
			loop = stmt
		}
	}
	if loop == nil {
		t.Fatal("no reusable loop obligation")
	}
	original := analyzer.resolvedForIterations[loop]
	assertUnproven := func(expected string) {
		t.Helper()
		err := analyzer.ValidateIteratorLoweringReadiness(program, "main")
		var readiness *IteratorLoweringReadinessError
		if !errors.As(err, &readiness) {
			t.Fatal(expected, err)
		}
		for _, issue := range readiness.Issues {
			if sameSourceToken(issue.Source, loop.Token) && issue.Classification == "Unproven" && strings.Contains(issue.Reason, expected) {
				return
			}
		}
		t.Fatal(expected, err)
	}
	delete(analyzer.resolvedForIterations, loop)
	assertUnproven("missing ResolvedForIteration")
	// An obligation remains even when the resolution producer loses its plan.
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "other"); err != nil {
		t.Fatal("unselected module blocked", err)
	}
	analyzer.resolvedForIterations[loop] = original
	for _, mutation := range []struct {
		name  string
		apply func(*ResolvedForIteration)
	}{
		{"conformance", func(plan *ResolvedForIteration) { plan.Conformance = Type{} }},
		{"yield type", func(plan *ResolvedForIteration) { plan.ElementType = Type{Kind: StringType, Name: "string"} }},
		{"source type", func(plan *ResolvedForIteration) { plan.SourceType = Type{Kind: StringType, Name: "string"} }},
		{"Next()", func(plan *ResolvedForIteration) { plan.NextCallable = "" }},
		{"binding", func(plan *ResolvedForIteration) { plan.Binding = ForIteratorDiscardBinding }},
		{"storage/lifetime", func(plan *ResolvedForIteration) { plan.Source = ForIteratorFreshTemporary }},
		{"advancement", func(plan *ResolvedForIteration) { plan.RequiresMutableReceiver = false }},
		{"reusable", func(plan *ResolvedForIteration) { plan.SourcePlace.Mutable = false }},
	} {
		plan := original
		mutation.apply(&plan)
		analyzer.resolvedForIterations[loop] = plan
		assertUnproven(mutation.name)
	}
	analyzer.resolvedForIterations[loop] = original
	dependencies := analyzer.iterationDependencies[loop]
	delete(analyzer.iterationDependencies, loop)
	assertUnproven("storage/lifetime")
	analyzer.iterationDependencies[loop] = dependencies
	graph := analyzer.callGraph
	analyzer.callGraph = nil
	assertUnproven("call-graph")
	analyzer.callGraph = graph
	sites := graph.sites
	graph.sites = nil
	assertUnproven("Next invocation")
	graph.sites = sites
	iterable := loop.Iterable
	loop.Iterable = &ast.Identifier{Value: "different"}
	assertUnproven("snapshot")
	loop.Iterable = iterable
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
}

// callableIDForName finds a test callable in the already-produced graph.
// Rules: rules/analysis/call_graph.md — "Callable node".
func callableIDForName(analyzer *Analyzer, name string) CallableID {
	for _, node := range analyzer.callGraph.Nodes() {
		if node.Name == name {
			return node.ID
		}
	}
	return ""
}

// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/control-flow/flowcontrol_for.md — §37.
func TestIteratorLoweringReadinessInvalidAndDeadSources(t *testing.T) {
	program := parameterRegressionProgram(t, "iterator_lowering_readiness_invalid")
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	var readiness *IteratorLoweringReadinessError
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); !errors.As(err, &readiness) {
		t.Fatal(err)
	}
	if len(readiness.Issues) != 4 {
		t.Fatal("invalid/dead obligations", readiness)
	}
	for _, issue := range readiness.Issues {
		if issue.Classification != "Invalid" {
			t.Fatal(issue)
		}
	}
	for stmt, requirement := range analyzer.iteratorLoweringRequirements {
		if !requirement.reachable {
			delete(analyzer.resolvedForIterations, stmt)
		}
	}
	for index := 0; index < 10; index++ {
		var next *IteratorLoweringReadinessError
		errors.As(analyzer.ValidateIteratorLoweringReadiness(program, "main"), &next)
		if !reflect.DeepEqual(readiness, next) {
			t.Fatal("map order changed readiness evidence")
		}
	}
	readiness.Issues[0].Reason = "changed snapshot"
	var next *IteratorLoweringReadinessError
	errors.As(analyzer.ValidateIteratorLoweringReadiness(program, "main"), &next)
	if next.Issues[0].Reason == "changed snapshot" {
		t.Fatal("error snapshot aliases analyzer")
	}
}

// Rules: rules/compiler/compiler_pipeline.md — §§33(1), 34(2–3).
func TestIteratorLoweringReadinessSnapshotReuse(t *testing.T) {
	program := parameterRegressionProgram(t, "iterator_lowering_readiness_valid")
	analyzer := NewAnalyzer()
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err == nil {
		t.Fatal("fresh analyzer accepted")
	}
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	other := parameterRegressionProgram(t, "iterator_lowering_readiness_valid")
	if err := analyzer.ValidateIteratorLoweringReadiness(other, "main"); err == nil {
		t.Fatal("other snapshot accepted")
	}
	if errs := analyzer.Analyze(other); len(errs) != 0 {
		t.Fatal(errs)
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err == nil {
		t.Fatal("old snapshot accepted after reuse")
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(other, "main"); err != nil {
		t.Fatal(err)
	}
	// Losing a built-in loop's absent protocol plan is not a protocol obligation.
	for stmt, requirement := range analyzer.iteratorLoweringRequirements {
		if !requirement.required {
			delete(analyzer.resolvedForIterations, stmt)
		}
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(other, "main"); err != nil {
		t.Fatal(err)
	}
}
