package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func parameterRegressionProgram(t *testing.T, name string) *ast.Program {
	t.Helper()
	source, err := os.ReadFile("../../testdata/sema/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "matrix.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

func parameterRegressionAnalyzer(t *testing.T, depth AnalysisDepth) *Analyzer {
	t.Helper()
	analyzer := NewAnalyzerWithDepth(depth)
	if errs := analyzer.Analyze(parameterRegressionProgram(t, "parameter_regression_matrix_valid")); len(errs) != 0 {
		t.Fatal(errs)
	}
	return analyzer
}

// Narrowing covers the integrated shared-reference policy and preserves fixed
// extent/ownership/address requirements. Specialized view/mutable candidates
// stay unavailable until their separately tracked producer is implemented.
// Rules: rules/analysis/parameter_usage_analysis.md — "Required narrowing tests",
// "Required positive/no-advisory tests", "False-positive regression tests".
func TestParameterRegressionNarrowingAndIntent(t *testing.T) {
	analyzer := parameterRegressionAnalyzer(t, AnalysisStandard)
	analysis := analyzer.ParameterUsageAnalysis()
	recommendations := map[string]ParameterRecommendation{}
	for _, recommendation := range analysis.Recommendations() {
		recommendations[recommendation.CallableName] = recommendation
	}
	for _, name := range []string{"Read", "Prefix"} {
		candidate := recommendations[name]
		if candidate.Status != "recommended" || candidate.Confidence != "strong" || len(candidate.Blockers) != 0 {
			t.Fatalf("%s: %+v", name, candidate)
		}
	}
	if recommendations["Prefix"].Candidate != "ref int[100]" {
		t.Fatal("fixed extent changed", recommendations["Prefix"])
	}
	for _, name := range []string{"CryptoBlock", "ProtocolHeader", "Snapshot", "Owning", "Identity", "MutableSequence", "ForeignBuffer"} {
		candidate := recommendations[name]
		if candidate.Status == "recommended" {
			t.Fatalf("intentional capability narrowed: %s %+v", name, candidate)
		}
	}
	for _, name := range []string{"CryptoBlock", "ProtocolHeader"} {
		demand := parameterUsageSummaryNamed(t, analysis, name).Parameters[0].Demand
		if !parameterDemandHasShape(demand, ParameterShapeExactExtent) || demand.Ownership != ParameterOwnershipRequired {
			t.Fatal(name, demand)
		}
	}
	for _, name := range []string{"Mutation", "MutableElements", "BorrowedElements"} {
		if _, exists := recommendations[name]; exists {
			t.Fatal("explicit borrow has replacement advice", name)
		}
	}
	if recommendations["Cheap"].Status != "not-preferred" && recommendations["Cheap"].Status != "blocked" {
		t.Fatal(recommendations["Cheap"])
	}
	if recommendations["Constrained"].Status == "recommended" {
		t.Fatal("cheap constrained aggregate recommended")
	}
	demand := parameterUsageSummaryNamed(t, analysis, "Prefix").Parameters[0].Demand
	if demand.MinimumExtent != 4 || parameterDemandHasShape(demand, ParameterShapeExactExtent) {
		t.Fatal("index inflated extent", demand)
	}
}

// SCC projection growth has a finite evidence domain and cannot become exact
// merely because recursion reaches its widening limit. Declaration order and
// map order cannot alter exported capability demand.
// Rules: rules/analysis/parameter_usage_analysis.md — "Recursive functions",
// "Required interprocedural tests", "Determinism".
func TestParameterRegressionProjectionWideningAndOrder(t *testing.T) {
	program := parameterRegressionProgram(t, "parameter_regression_matrix_valid")
	analyze := func(depth AnalysisDepth) *ParameterUsageAnalysis {
		analyzer := NewAnalyzerWithDepth(depth)
		if errs := analyzer.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		return analyzer.ParameterUsageAnalysis()
	}
	before := analyze(AnalysisStandard)
	for _, name := range []string{"Recursive", "Alpha", "Beta"} {
		parameter := parameterUsageSummaryNamed(t, before, name).Parameters[0]
		if parameter.Demand.Precision != ParameterDemandPartial {
			t.Fatal(name, "recursive precision", parameter.Demand)
		}
		maximum := 0
		for _, use := range parameter.Uses {
			if len(use.Place.Projections) > maximum {
				maximum = len(use.Place.Projections)
			}
		}
		if maximum != parameterUsageProjectionLimit {
			t.Fatal(name, "projection bound", maximum)
		}
	}
	if _, converged := before.InterproceduralStatus(); !converged {
		t.Fatal("bounded projections failed to converge")
	}

	// Reverse callable registration while retaining each declaration's canonical tokens.
	var positions []int
	for index, statement := range program.Statements {
		if _, ok := statement.(*ast.FunctionDeclaration); ok {
			positions = append(positions, index)
		}
	}
	for i, j := 0, len(positions)-1; i < j; i, j = i+1, j-1 {
		program.Statements[positions[i]], program.Statements[positions[j]] = program.Statements[positions[j]], program.Statements[positions[i]]
	}
	after := analyze(AnalysisStandard)
	for _, summary := range before.Summaries() {
		other, ok := after.Summary(summary.Callable)
		if !ok || len(other.Parameters) != len(summary.Parameters) {
			t.Fatal("summary identity changed", summary.Name)
		}
		for index, parameter := range summary.Parameters {
			if !parameterDemandsEqual(parameter.Demand, other.Parameters[index].Demand) {
				t.Fatal("declaration order changed demand", summary.Name)
			}
		}
	}
	interactive := analyze(AnalysisInteractive)
	for _, name := range []string{"Recursive", "Alpha", "Beta"} {
		parameter := parameterUsageSummaryNamed(t, interactive, name).Parameters[0]
		if parameter.Demand.Precision == ParameterDemandExact || sharedReferenceRecommendation(parameter).Status == "recommended" {
			t.Fatal("budget exhaustion strengthened recursive proof", name)
		}
	}
}

// Unknown FFI/callable contracts must block positive advice; known address and
// contiguous-storage evidence survives beside uncertainty. No pointer/extent
// relationship is inferred from a foreign parameter's name.
// Rules: rules/analysis/parameter_usage_analysis.md — "Required FFI tests",
// "Missing foreign retention information", "Unknown critical dimensions block narrowing".
func TestParameterRegressionForeignUncertainty(t *testing.T) {
	analysis := parameterRegressionAnalyzer(t, AnalysisDeep).ParameterUsageAnalysis()
	for _, name := range []string{"Foreign", "ForeignBuffer", "ForeignCallback"} {
		summary := parameterUsageSummaryNamed(t, analysis, name)
		for _, parameter := range summary.Parameters {
			if name == "ForeignCallback" && parameter.Name == "operation" {
				if parameter.Demand.Access != ParameterAccessRead {
					t.Fatal("invoked callable is unused", parameter.Demand)
				}
				continue
			}
			if parameter.Demand.Ownership != ParameterUnknownOwnership || parameter.Demand.Precision == ParameterDemandExact {
				t.Fatal(name, parameter.Name, parameter.Demand)
			}
			if sharedReferenceRecommendation(parameter).Status == "recommended" {
				t.Fatal("unknown foreign/callback capability recommended", name)
			}
		}
	}
	buffer := parameterUsageSummaryNamed(t, analysis, "ForeignBuffer").Parameters[0]
	if !parameterDemandHasShape(buffer.Demand, ParameterShapeContiguousSequence) || !hasSpecialParameterStorage(buffer.Demand.Storage) {
		t.Fatal("known pointer/storage evidence lost", buffer.Demand)
	}
}

// Demand belongs to a resolved binding, never another callable's same-spelled
// parameter. Invalid lexical redeclarations keep their owning Sema diagnostic.
// Rules: rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses",
// "Required positive/no-advisory tests"; rules/analysis/closure_analysis.md — "Abstract closure identity".
func TestParameterRegressionNestedBindingIdentity(t *testing.T) {
	analyzer := parameterRegressionAnalyzer(t, AnalysisDeep)
	frame := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "NestedParameter").Parameters[0]
	if frame.Demand.Access != ParameterAccessUnused || len(frame.Uses) != 0 {
		t.Fatalf("lambda parameter attributed to enclosing frame: %+v", frame.Demand)
	}
	for _, name := range []string{"CopyCapture", "MoveCapture"} {
		captured := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), name).Parameters[0]
		if captured.Demand.Access != ParameterAccessRead {
			t.Fatal("capture creation lost source read", name, captured.Demand)
		}
		if name == "MoveCapture" && captured.Demand.Ownership != ParameterConsumptionRequired {
			t.Fatal("capture move lost consumption", captured.Demand)
		}
	}
	invalid := NewAnalyzerWithDepth(AnalysisDeep)
	if errs := invalid.Analyze(parameterRegressionProgram(t, "parameter_regression_matrix_invalid")); len(errs) == 0 {
		t.Fatal("invalid shadowing lost owning diagnostic")
	}
}

// Optional recommendation production and cost changes must not mutate demand.
// Each critical capability and uncertainty dimension independently blocks the
// shared-reference candidate, including constraints not yet sourced from FFI.
// Rules: rules/analysis/parameter_usage_analysis.md — "Required recommendation-policy tests",
// "Candidate blockers", "Semantic demand and recommendation policy are separate".
func TestParameterRegressionRecommendationPolicyIsolation(t *testing.T) {
	analysis := parameterRegressionAnalyzer(t, AnalysisDeep).ParameterUsageAnalysis()
	before := analysis.Summaries()
	_ = analysis.Recommendations()
	if !reflect.DeepEqual(before, analysis.Summaries()) {
		t.Fatal("recommendation query changed semantic facts")
	}
	read := parameterUsageSummaryNamed(t, analysis, "Read").Parameters[0]
	cases := map[string]func(*ParameterDemand){
		"mutation":        func(d *ParameterDemand) { d.Mutation = ParameterElementOrFieldMutation },
		"retention":       func(d *ParameterDemand) { d.Lifetime = ParameterLifetimeRetained },
		"thread":          func(d *ParameterDemand) { d.Lifetime = ParameterLifetimeCrossThread },
		"consume":         func(d *ParameterDemand) { d.Ownership = ParameterConsumptionRequired },
		"stable identity": func(d *ParameterDemand) { d.Identity = ParameterStableIdentityRequired },
		"pinning":         func(d *ParameterDemand) { d.Storage = []ParameterStorageDemand{ParameterStoragePinned} },
		"alignment":       func(d *ParameterDemand) { d.Storage = []ParameterStorageDemand{ParameterStorageAligned} },
		"memory space":    func(d *ParameterDemand) { d.Storage = []ParameterStorageDemand{ParameterStorageMemorySpace} },
		"representation":  func(d *ParameterDemand) { d.Representation = ParameterRepresentationExact },
		"unknown shape":   func(d *ParameterDemand) { d.Shapes = []ParameterShapeDemand{ParameterShapeUnknown} },
	}
	for name, mutate := range cases {
		parameter := cloneParameterUsageParameterSummary(read)
		mutate(&parameter.Demand)
		result := sharedReferenceRecommendation(parameter)
		if result.Status != "blocked" || len(result.Blockers) == 0 || result.Confidence != "" {
			t.Fatal(name, result)
		}
	}
	small := cloneParameterUsageParameterSummary(read)
	for index := range small.DeclaredType.Fields {
		small.DeclaredType.Fields[index].Type = builtinType("int8")
	}
	if sharedReferenceRecommendation(small).Status == "recommended" {
		t.Fatal("cost policy ignores smaller copy")
	}
	if !parameterDemandsEqual(small.Demand, read.Demand) {
		t.Fatal("cost changed demand")
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		summary := parameterUsageSummaryNamed(t, parameterRegressionAnalyzer(t, depth).ParameterUsageAnalysis(), "Read")
		if !parameterDemandsEqual(read.Demand, summary.Parameters[0].Demand) {
			t.Fatal("depth changed nonrecursive semantic demand", depth)
		}
	}
	if !strings.Contains(sharedReferenceRecommendation(read).Candidate, "Frame") {
		t.Fatal("candidate identity lost")
	}
}
