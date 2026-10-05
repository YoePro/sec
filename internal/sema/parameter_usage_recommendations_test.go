package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestParameterRecommendations verifies policy output against resolved local
// and transitive demand, plus defensive snapshots and unchanged semantic facts.
// Rules: rules/analysis/parameter_usage_analysis.md — "Recommendation confidence",
// "Recommendation reasons", "Candidate blockers", and "Calls propagate demand".
func TestParameterRecommendations(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_recommendations_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	analysis := analyzer.ParameterUsageAnalysis()
	before := analysis.Summaries()
	results := analysis.Recommendations()
	byName := map[string]ParameterRecommendation{}
	for _, result := range results {
		byName[result.CallableName] = result
	}
	for _, name := range []string{"Read", "ForwardRead", "First"} {
		result := byName[name]
		if result.Status != "recommended" || result.Confidence != "strong" || !result.SizeKnown || result.EstimatedSizeBytes < 64 || len(result.Blockers) != 0 || len(result.Reasons) == 0 || result.Reasons[0] != "AvoidCopyCost" {
			t.Fatalf("%s recommendation = %#v", name, result)
		}
	}
	if byName["First"].Candidate != "ref int[100]" {
		t.Fatalf("fixed extent lost: %#v", byName["First"])
	}
	for _, name := range []string{"Sink", "ForwardSink"} {
		result := byName[name]
		if result.Status != "blocked" || result.Confidence != "" || !strings.Contains(strings.Join(result.Blockers, ","), "consumption") {
			t.Fatalf("%s recommendation = %#v", name, result)
		}
	}
	if result := byName["Small"]; result.Status != "not-preferred" || result.Confidence != "" || len(result.Reasons) == 0 {
		t.Fatalf("small value: %#v", result)
	}
	if _, ok := byName["Borrowed"]; ok {
		t.Fatal("already borrowed parameter received a candidate")
	}
	if !reflect.DeepEqual(before, analysis.Summaries()) {
		t.Fatal("policy changed semantic demand")
	}
	var advisoryCount int
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.LargeValueParameter {
			advisoryCount++
		}
	}
	if advisoryCount != 3 {
		t.Fatalf("got %d A2001 advisories, want 3", advisoryCount)
	}
	results[0].Reasons[0] = "changed"
	for i := range results {
		if len(results[i].Blockers) > 0 {
			results[i].Blockers[0] = "changed"
		}
	}
	fresh := analysis.Recommendations()
	for _, result := range fresh {
		if strings.Contains(strings.Join(append(result.Reasons, result.Blockers...), ","), "changed") {
			t.Fatal("recommendation slices alias snapshot")
		}
	}
	if !reflect.DeepEqual(fresh, analyzer.ParameterUsageAnalysis().Recommendations()) {
		t.Fatal("recommendation order/content is not stable")
	}
	// The advisory emitter requires exact callable precision, even when one
	// particular parameter stayed exact. Policy output must honor that gate.
	read := parameterUsageSummaryNamed(t, analysis, "Read")
	analysis.summaries[read.Callable].Precision = ParameterDemandPartial
	for _, result := range analysis.Recommendations() {
		if result.CallableName == "Read" && (result.Status != "blocked" || result.Confidence != "" || !strings.Contains(strings.Join(result.Blockers, ","), "callable demand precision is partial")) {
			t.Fatalf("non-exact callable received advice: %#v", result)
		}
	}
	if !reflect.DeepEqual(fresh, analyzer.ParameterUsageAnalysis().Recommendations()) {
		t.Fatal("snapshot edits changed analyzer policy output")
	}
}

// TestParameterRecommendationUnknownDimensions verifies localized unknowns
// cannot yield a recommended or confident candidate, even for a large type.
// Rules: rules/analysis/parameter_usage_analysis.md — "Unknown critical dimensions block narrowing".
func TestParameterRecommendationUnknownDimensions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_recommendations_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	parameter := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Read").Parameters[0]
	tests := []struct {
		name    string
		change  func(*ParameterDemand)
		blocker string
	}{
		{"precision", func(d *ParameterDemand) { d.Precision = ParameterDemandPartial }, "precision is partial"},
		{"access", func(d *ParameterDemand) { d.Access = ParameterAccessUnknown }, "access demand is unknown"},
		{"mutation", func(d *ParameterDemand) { d.Mutation = ParameterUnknownMutation }, "mutation demand is unknown"},
		{"ownership", func(d *ParameterDemand) { d.Ownership = ParameterUnknownOwnership }, "ownership demand is unknown"},
		{"lifetime", func(d *ParameterDemand) { d.Lifetime = ParameterLifetimeUnknown }, "lifetime demand is unknown"},
		{"identity", func(d *ParameterDemand) { d.Identity = ParameterUnknownIdentity }, "identity demand is unknown"},
		{"representation", func(d *ParameterDemand) { d.Representation = ParameterRepresentationUnknown }, "representation demand is unknown"},
		{"storage", func(d *ParameterDemand) { d.Storage = []ParameterStorageDemand{ParameterStorageUnknown} }, "storage demand is unknown"},
		{"shape", func(d *ParameterDemand) { d.Shapes = []ParameterShapeDemand{ParameterShapeUnknown} }, "shape demand is unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := parameter
			copy.Demand = cloneParameterDemand(parameter.Demand)
			tt.change(&copy.Demand)
			result := sharedReferenceRecommendation(copy)
			if result.Status != "blocked" || result.Confidence != "" || !strings.Contains(strings.Join(result.Blockers, ","), tt.blocker) {
				t.Fatalf("result = %#v", result)
			}
		})
	}
	parameter.Demand.Shapes = append(parameter.Demand.Shapes, ParameterShapeExactExtent)
	if result := sharedReferenceRecommendation(parameter); result.Status != "recommended" {
		t.Fatalf("same-type reference must preserve extent: %#v", result)
	}
	parameter.DeclaredType = Type{Kind: GenericType, Name: "T"}
	if result := sharedReferenceRecommendation(parameter); result.Status != "not-preferred" || result.SizeKnown {
		t.Fatalf("unknown cost produced advice: %#v", result)
	}
}
