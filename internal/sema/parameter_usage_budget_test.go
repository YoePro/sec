package sema

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"sec/internal/diagnostics"
)

// Rules: rules/analysis/parameter_usage_analysis.md — "Analysis budgets",
// "Interactive analysis", "Standard analysis", "Deep analysis";
// rules/compiler/compiler_analysis.md — §15(1–5).
func TestParameterBudgetDepthsAndConfiguration(t *testing.T) {
	var previous ParameterUsageBudget
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer := NewAnalyzerWithDepth(depth)
		budget := analyzer.ParameterUsageBudget()
		if budget.MaxSummaryIterations <= previous.MaxSummaryIterations || budget.MaxRecommendationWork <= previous.MaxRecommendationWork || budget.MaxTypeDepth <= previous.MaxTypeDepth || budget.MaxPersistenceRecords <= previous.MaxPersistenceRecords || budget.MaxPersistenceBytes <= previous.MaxPersistenceBytes {
			t.Fatal("depth budgets do not increase", previous, budget)
		}
		previous = budget
		for _, field := range []string{"iterations", "work", "depth", "records", "bytes"} {
			invalid := budget
			switch field {
			case "iterations":
				invalid.MaxSummaryIterations = -1
			case "work":
				invalid.MaxRecommendationWork = -1
			case "depth":
				invalid.MaxTypeDepth = -1
			case "records":
				invalid.MaxPersistenceRecords = -1
			case "bytes":
				invalid.MaxPersistenceBytes = -1
			}
			if err := analyzer.SetParameterUsageBudget(invalid); err == nil {
				t.Fatal("negative budget accepted", field)
			}
			if analyzer.ParameterUsageBudget() != budget {
				t.Fatal("rejected budget changed policy")
			}
		}
	}
}

// Rules: rules/compiler/compiler_analysis.md — §15(3–5);
// rules/analysis/parameter_usage_analysis.md — "Analysis budgets", "Determinism",
// "Semantic demand and recommendation policy are separate".
func TestParameterBudgetRecommendationAdmission(t *testing.T) {
	analyzer := NewAnalyzer()
	budget := analyzer.ParameterUsageBudget()
	budget.MaxRecommendationWork = 3
	if err := analyzer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	program := parameterRegressionProgram(t, "parameter_budget_valid")
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	snapshot := analyzer.ParameterUsageAnalysis()
	before := snapshot.Summaries()
	recommendations, coverage := snapshot.RecommendationsWithCoverage()
	if len(recommendations) != 1 || recommendations[0].CallableName != "First" || recommendations[0].Status != "recommended" || coverage.EvaluatedCandidates != 1 || coverage.SkippedCandidates != 2 || coverage.VisitedWork != 3 {
		t.Fatal(recommendations, coverage)
	}
	count := 0
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.LargeValueParameter {
			count++
		}
	}
	if count != 1 {
		t.Fatal("advisory emitter bypassed recommendation budget", count)
	}
	repeated, repeatedCoverage := snapshot.RecommendationsWithCoverage()
	if !reflect.DeepEqual(recommendations, repeated) || coverage != repeatedCoverage || !reflect.DeepEqual(before, snapshot.Summaries()) {
		t.Fatal("budget evaluation changed demand or was nondeterministic")
	}
	// Registration order does not change which complete candidate is admitted.
	program.Statements[1], program.Statements[3] = program.Statements[3], program.Statements[1]
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	reordered, reorderedCoverage := analyzer.ParameterUsageAnalysis().RecommendationsWithCoverage()
	// Binding IDs belong to each source analysis, while candidate identity and
	// admission order are determined by canonical source tokens.
	for index := range reordered {
		if index < len(recommendations) {
			reordered[index].Binding = recommendations[index].Binding
		}
	}
	if !reflect.DeepEqual(recommendations, reordered) || coverage != reorderedCoverage {
		t.Fatal("registration order changed budgeted advice")
	}
	// Snapshot policy is detached; zero optional work affects only the next run.
	budget.MaxRecommendationWork = 0
	if err := analyzer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	old, oldCoverage := snapshot.RecommendationsWithCoverage()
	if !reflect.DeepEqual(old, recommendations) || oldCoverage != coverage {
		t.Fatal("new configuration mutated prior snapshot")
	}
	if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	disabled, disabledCoverage := analyzer.ParameterUsageAnalysis().RecommendationsWithCoverage()
	if len(disabled) != 0 || disabledCoverage.SkippedCandidates != 3 || disabledCoverage.VisitedWork != 0 {
		t.Fatal(disabled, disabledCoverage)
	}
	if !reflect.DeepEqual(before, analyzer.ParameterUsageAnalysis().Summaries()) {
		t.Fatal("optional budget changed semantic demand")
	}
}

// Rules: rules/compiler/compiler_analysis.md — §§14(3), 15(3–5);
// rules/analysis/parameter_usage_analysis.md — "Recursive functions", "Analysis budgets".
func TestParameterBudgetIterationWideningAndSourceValidity(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer := NewAnalyzerWithDepth(depth)
		budget := analyzer.ParameterUsageBudget()
		budget.MaxSummaryIterations = 1
		budget.MaxRecommendationWork = 0
		if err := analyzer.SetParameterUsageBudget(budget); err != nil {
			t.Fatal(err)
		}
		if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_callable_targets_valid")); len(errors) != 0 {
			t.Fatal(errors)
		}
		snapshot := analyzer.ParameterUsageAnalysis()
		iterations, converged := snapshot.InterproceduralStatus()
		if iterations != 1 || converged {
			t.Fatal(depth, iterations, converged)
		}
		recursive := parameterUsageSummaryNamed(t, snapshot, "Recursive").Parameters[0].Demand
		if recursive.Precision != ParameterDemandUnknown || recursive.Access != ParameterAccessUnknown || recursive.Lifetime != ParameterLifetimeUnknown {
			t.Fatal("budget became a positive proof", depth, recursive)
		}
		// Optional work exhaustion never hides the owning bool/integer error.
		errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_budget_invalid"))
		if len(errors) != 1 {
			t.Fatal(depth, errors)
		}
		baseline := NewAnalyzerWithDepth(depth)
		want := baseline.Analyze(parameterRegressionProgram(t, "parameter_budget_invalid"))
		if !reflect.DeepEqual(errors, want) {
			t.Fatal("budget changed source validity", depth, errors, want)
		}
	}
}

// Rules: rules/compiler/compiler_analysis.md — §15(2–5);
// rules/analysis/parameter_usage_analysis.md — "Analysis budgets", "ResolvedLayout as cost input".
func TestParameterBudgetTypeDepthAndCycles(t *testing.T) {
	analyzer := NewAnalyzer()
	budget := analyzer.ParameterUsageBudget()
	budget.MaxTypeDepth = 1
	if err := analyzer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	recommendations, coverage := analyzer.ParameterUsageAnalysis().RecommendationsWithCoverage()
	if len(recommendations) != 0 || coverage.SkippedCandidates != 3 {
		t.Fatal("partial type inspection authorized advice", recommendations, coverage)
	}
	// Malformed cyclic metadata cannot make optional preflight recurse forever.
	typ := Type{Kind: ArrayType}
	typ.Element = &typ
	for _, transport := range []bool{false, true} {
		work := parameterWorkBudget{remaining: 100, maxDepth: 64}
		if work.typeFits(typ, 1, transport, map[*Type]bool{}) {
			t.Fatal("cyclic type preflight admitted")
		}
		if work.visited > 100 {
			t.Fatal("preflight exceeded work budget")
		}
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation",
// "Persisted summary versioning", "Analysis budgets";
// rules/compiler/compiler_analysis.md — §15(3–5).
func TestParameterBudgetPersistenceAtomicityAndBoundaries(t *testing.T) {
	producer := NewAnalyzer()
	if errors := producer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	snapshot := producer.ParameterUsageAnalysis()
	identities := map[CallableID]ParameterDemandSummaryIdentity{}
	for _, summary := range snapshot.Summaries() {
		identities[summary.Callable] = ParameterDemandSummaryIdentity{Specialization: "template", CompilerModel: "model", DependencyFingerprint: "inputs"}
	}
	data, err := snapshot.MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []string{"records", "bytes", "depth", "disabled"} {
		t.Run(limit, func(t *testing.T) {
			budget := parameterUsageBudget(AnalysisStandard)
			switch limit {
			case "records":
				budget.MaxPersistenceRecords = 1
			case "bytes":
				budget.MaxPersistenceBytes = len(data) - 1
			case "depth":
				budget.MaxTypeDepth = 1
			case "disabled":
				budget.MaxPersistenceBytes = 0
			}
			if err := producer.SetParameterUsageBudget(budget); err != nil {
				t.Fatal(err)
			}
			if errors := producer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errors) != 0 {
				t.Fatal(errors)
			}
			limited, err := producer.ParameterUsageAnalysis().MarshalDemandSummaries(identities)
			var exhausted *ParameterUsageBudgetError
			if !errors.As(err, &exhausted) || limited != nil {
				t.Fatal("partial artifact published", limit, len(limited), err)
			}
			// Install good metadata first, then verify that failed replacement clears it.
			consumer := NewAnalyzer()
			if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
				t.Fatal(err)
			}
			if err := consumer.SetParameterUsageBudget(budget); err != nil {
				t.Fatal(err)
			}
			if len(consumer.importedParameterDemands) != 0 {
				t.Fatal("policy change retained old admission")
			}
			importErr := consumer.SetImportedParameterDemands(data, identities)
			if limit == "depth" {
				// Input signatures are hashed; depth work occurs when matching current types.
				if importErr != nil {
					t.Fatal(importErr)
				}
				// Directly exercise installation with current declarations to isolate the
				// signature budget from the source loader's required function-body checks.
				consumer.functions = producer.functions
				result := newParameterUsageAnalysis()
				result.budget = budget
				builder := parameterUsageBuilder{analyzer: consumer, result: result}
				builder.installImportedDemands()
				if len(result.summaries) != 0 || result.ImportCoverage().SkippedCallables != 3 {
					t.Fatal("signature depth exhaustion imported positive facts", result.ImportCoverage())
				}
			} else if !errors.As(importErr, &exhausted) || len(consumer.importedParameterDemands) != 0 {
				t.Fatal("partial import survived", limit, importErr)
			}
			if err := consumer.SetImportedParameterDemands(nil, nil); err != nil {
				t.Fatal("clearing metadata used optional budget", err)
			}
		})
	}
	// Exact byte boundary succeeds with byte-identical artifacts, independent of depth.
	budget := parameterUsageBudget(AnalysisStandard)
	budget.MaxPersistenceBytes = len(data)
	if err := producer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	if errs := producer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errs) != 0 {
		t.Fatal(errs)
	}
	exact, err := producer.ParameterUsageAnalysis().MarshalDemandSummaries(identities)
	if err != nil || !reflect.DeepEqual(exact, data) {
		t.Fatal("exact export byte limit failed", err)
	}
	consumer := NewAnalyzer()
	if err := consumer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
		t.Fatal("exact import byte limit failed", err)
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Determinism",
// "Function summaries", "Analysis budgets"; rules/compiler/compiler_analysis.md — immutable results.
func TestParameterBudgetConcurrentSnapshotQueries(t *testing.T) {
	analyzer := NewAnalyzerWithDepth(AnalysisDeep)
	budget := analyzer.ParameterUsageBudget()
	budget.MaxRecommendationWork = 3
	if err := analyzer.SetParameterUsageBudget(budget); err != nil {
		t.Fatal(err)
	}
	if errs := analyzer.Analyze(parameterRegressionProgram(t, "parameter_budget_valid")); len(errs) != 0 {
		t.Fatal(errs)
	}
	snapshot := analyzer.ParameterUsageAnalysis()
	recommendations, coverage := snapshot.RecommendationsWithCoverage()
	identities := map[CallableID]ParameterDemandSummaryIdentity{}
	for _, summary := range snapshot.Summaries() {
		identities[summary.Callable] = ParameterDemandSummaryIdentity{Specialization: "template", CompilerModel: "model", DependencyFingerprint: "current"}
	}
	artifact, err := snapshot.MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := 0; iteration < 8; iteration++ {
				got, gotCoverage := snapshot.RecommendationsWithCoverage()
				if !reflect.DeepEqual(got, recommendations) || gotCoverage != coverage {
					t.Error("concurrent query changed candidate admission")
				}
				data, err := snapshot.MarshalDemandSummaries(identities)
				if err != nil || !reflect.DeepEqual(data, artifact) {
					t.Error("concurrent query changed persisted artifact", err)
				}
			}
		}()
	}
	workers.Wait()
}
