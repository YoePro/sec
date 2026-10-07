package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
)

// Rules: rules/analysis/parameter_usage_analysis.md — "Function-value calls",
// "Calls propagate demand", "Dimension-specific unknown", "Required interprocedural tests".
func TestParameterCallableTargets(t *testing.T) {
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_callable_targets_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	analysis := analyzer.ParameterUsageAnalysis()
	get := func(name, parameter string) ParameterUsageParameterSummary {
		return parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, name), parameter)
	}
	hasUse := func(parameter ParameterUsageParameterSummary, place string) bool {
		for _, use := range parameter.Uses {
			if use.Kind == ParameterUseCall && use.Place.String() == place {
				return true
			}
		}
		return false
	}
	for _, name := range []string{"Named", "Joined", "Rebound"} {
		parameter := get(name, "pair")
		if parameter.Demand.Precision != ParameterDemandExact || parameter.Demand.Lifetime == ParameterLifetimeUnknown || !hasUse(parameter, "pair.first") {
			t.Fatal(name, parameter)
		}
	}
	if !hasUse(get("Joined", "pair"), "pair.second") {
		t.Fatal("joined target demand lost")
	}
	joinedLambda := get("JoinedLambda", "pair")
	if joinedLambda.Demand.Precision != ParameterDemandPartial || !hasUse(joinedLambda, "pair.first") || !hasUse(joinedLambda, "pair.second") {
		t.Fatal("joined lambda targets not consumed", joinedLambda)
	}
	for _, name := range []string{"Open", "Mixed"} {
		demand := get(name, "pair").Demand
		if demand.Access != ParameterAccessRead || demand.Mutation != ParameterNoMutation || demand.Ownership != ParameterBorrowSufficient || demand.Identity != ParameterAddressRequired || demand.Lifetime != ParameterLifetimeUnknown || demand.Storage[0] != ParameterStorageUnknown || demand.Representation != ParameterRepresentationUnknown || demand.Precision != ParameterDemandPartial {
			t.Fatal(name, demand)
		}
	}
	if !hasUse(get("Mixed", "pair"), "pair.first") || !hasUse(get("Mixed", "pair"), "pair.second") {
		t.Fatal("open alternatives erased known projection evidence")
	}
	sequence := get("OpenSequence", "values").Demand
	if sequence.MinimumExtent != 4 || !hasParameterShape(sequence.Shapes, ParameterShapeRandomAccess) || !hasParameterShape(sequence.Shapes, ParameterShapeUnknown) || sequence.Lifetime != ParameterLifetimeUnknown {
		t.Fatal("open boundary erased independently known sequence facts", sequence)
	}
	mutable := get("Mutable", "pair").Demand
	if mutable.Mutation != ParameterElementOrFieldMutation || mutable.Precision != ParameterDemandExact {
		t.Fatal(mutable)
	}
	openMutable := get("OpenMutable", "pair").Demand
	if openMutable.Mutation != ParameterUnknownMutation || openMutable.Ownership != ParameterBorrowSufficient || openMutable.Identity != ParameterAddressRequired {
		t.Fatal(openMutable)
	}
	for _, name := range []string{"Lambda", "Capturing", "Nested"} {
		parameter := get(name, "pair")
		if parameter.Demand.Precision != ParameterDemandPartial || parameter.Demand.Lifetime != ParameterLifetimeUnknown || parameter.Demand.Ownership != ParameterUnknownOwnership {
			t.Fatal(name, parameter)
		}
		projection := "pair.second"
		if name == "Capturing" {
			projection = "pair.first"
		}
		if !hasUse(parameter, projection) {
			t.Fatal(name, "missing lambda projection", parameter.Uses)
		}
	}
	scalar := get("ScalarLambda", "value").Demand
	if scalar.Precision != ParameterDemandExact || scalar.Lifetime != ParameterLifetimeCallOnly {
		t.Fatal("known scalar lambda unnecessarily widened", scalar)
	}
	if get("Capturing", "bias").Demand.Precision != ParameterDemandPartial {
		t.Fatal("capture creation lost conservative environment demand")
	}
	borrowed := get("LambdaBorrow", "pair")
	if borrowed.Demand.Lifetime != ParameterLifetimeUnknown || !hasUse(borrowed, "pair.first") {
		t.Fatal("missing lambda escape facts became a lifetime proof", borrowed)
	}
	recursive := get("Recursive", "node")
	if recursive.Demand.Precision != ParameterDemandPartial {
		t.Fatal("recursive projection growth not widened", recursive)
	}
	if _, converged := analysis.InterproceduralStatus(); !converged {
		t.Fatal("callable target joins did not converge")
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Recursive functions",
// "Determinism"; rules/compiler/compiler_analysis.md — immutable results.
func TestParameterCallableOrderAndSnapshots(t *testing.T) {
	program := parameterRegressionProgram(t, "parameter_callable_targets_valid")
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	before := analyzer.ParameterUsageAnalysis()
	snapshot := parameterUsageSummaryNamed(t, before, "Joined")
	snapshot.Parameters[1].Uses = nil
	snapshot.Parameters[1].Demand.Shapes = nil
	if reflect.DeepEqual(snapshot, parameterUsageSummaryNamed(t, before, "Joined")) {
		t.Fatal("summary aliases snapshot")
	}
	var positions []int
	for index, statement := range program.Statements {
		if _, ok := statement.(*ast.FunctionDeclaration); ok {
			positions = append(positions, index)
		}
	}
	for i, j := 0, len(positions)-1; i < j; i, j = i+1, j-1 {
		program.Statements[positions[i]], program.Statements[positions[j]] = program.Statements[positions[j]], program.Statements[positions[i]]
	}
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	after := analyzer.ParameterUsageAnalysis()
	for _, summary := range before.Summaries() {
		other, ok := after.Summary(summary.Callable)
		if !ok || len(summary.Parameters) != len(other.Parameters) {
			t.Fatal("registration order changed callable identity", summary.Name)
		}
		for index, parameter := range summary.Parameters {
			if !parameterDemandsEqual(parameter.Demand, other.Parameters[index].Demand) {
				t.Fatal("registration order changed callable demand", summary.Name, parameter.Demand, other.Parameters[index].Demand)
			}
			if len(parameter.Uses) != len(other.Parameters[index].Uses) {
				t.Fatal("registration order changed evidence", summary.Name)
			}
			for useIndex, use := range parameter.Uses {
				otherUse := other.Parameters[index].Uses[useIndex]
				if use.Place.String() != otherUse.Place.String() || use.Kind != otherUse.Kind || !sameSourceToken(use.Source, otherUse.Source) {
					t.Fatal("registration order changed projected evidence", summary.Name, use, otherUse)
				}
			}
		}
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Unknown critical dimensions block narrowing";
// rules/declarations/functions.md — argument compatibility.
func TestParameterCallableInvalidInvocation(t *testing.T) {
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_callable_targets_invalid")); len(errors) == 0 {
		t.Fatal("invalid argument accepted")
	}
	parameter := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Invalid"), "value")
	if parameter.Demand.Precision == ParameterDemandExact {
		t.Fatal("invalid invocation produced positive target proof", parameter)
	}
	dead := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Dead"), "pair")
	if dead.Demand.Precision != ParameterDemandExact || dead.Demand.Lifetime == ParameterLifetimeUnknown {
		t.Fatal("unreachable open call contributed demand", dead)
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Function-value calls",
// "Dimension-specific unknown", "Unknown critical dimensions block narrowing".
func TestParameterCallableOpenModesAndMissingFacts(t *testing.T) {
	contract := &OpenCallableContract{Parameters: []CallableParameterContract{
		{Ref: true}, {Consuming: true}, {Ref: true, MutableRef: true, Variadic: true},
	}}
	consumed := openCallableParameterDemand(contract, 1)
	if consumed.Ownership != ParameterConsumptionRequired || consumed.Lifetime != ParameterLifetimeUnknown || !hasParameterShape(consumed.Shapes, ParameterShapeWholeValue) {
		t.Fatal("consumption contract weakened", consumed)
	}
	variadic := openCallableParameterDemand(contract, 4)
	if variadic.Ownership != ParameterBorrowSufficient || variadic.Identity != ParameterAddressRequired || variadic.Mutation != ParameterUnknownMutation {
		t.Fatal("variadic invocation lost reference mode", variadic)
	}
	for _, demand := range []ParameterDemand{openCallableParameterDemand(nil, 0), openCallableParameterDemand(contract, -1), openCallableParameterDemand(&OpenCallableContract{}, 0)} {
		if demand.Precision != ParameterDemandUnknown || demand.Ownership != ParameterUnknownOwnership || demand.Lifetime != ParameterLifetimeUnknown {
			t.Fatal("missing boundary facts created proof", demand)
		}
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Recursive functions",
// "Unknown critical dimensions block narrowing"; rules/compiler/compiler_analysis.md — §14(3).
func TestParameterCallableBudgetWidening(t *testing.T) {
	analyzer := NewAnalyzerWithDepth(AnalysisInteractive)
	analyzer.analysisBudget.MaxSummaryIterations = 1
	if errors := analyzer.Analyze(parameterRegressionProgram(t, "parameter_callable_targets_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	analysis := analyzer.ParameterUsageAnalysis()
	if _, converged := analysis.InterproceduralStatus(); converged {
		t.Fatal("single iteration claimed convergence")
	}
	for _, name := range []string{"Joined", "Mixed", "Lambda", "Nested", "Recursive"} {
		parameterName := "pair"
		if name == "Recursive" {
			parameterName = "node"
		}
		parameter := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, name), parameterName)
		if parameter.Demand.Precision != ParameterDemandUnknown {
			t.Fatal("budget created positive callable proof", name, parameter.Demand)
		}
	}
}
