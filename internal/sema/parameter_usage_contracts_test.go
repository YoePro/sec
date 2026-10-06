package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestParameterContracts preserves explicit cleanup ownership separately from
// body demand, borrowed subvalues, noCopy, consumption, and fixed array extent.
// Rules: rules/analysis/parameter_usage_analysis.md — "Explicit contracts and programmer intent";
// rules/memory/destruction.md — §§3.3–3.4, 4(1–5).
func TestParameterContracts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_contracts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errs, nil)
	analysis := analyzer.ParameterUsageAnalysis()
	before := analysis.Summaries()
	results := map[string]ParameterRecommendation{}
	for _, result := range analysis.Recommendations() {
		results[result.CallableName] = result
	}
	for _, name := range []string{"Read", "Nested", "Elements"} {
		result := results[name]
		if result.Status != "blocked" || result.Confidence != "" || !strings.Contains(strings.Join(result.Blockers, ","), "explicit free lifecycle") {
			t.Fatalf("%s lost cleanup contract: %+v", name, result)
		}
		if parameterUsageSummaryNamed(t, analysis, name).Parameters[0].Demand.Ownership != ParameterBorrowSufficient {
			t.Fatal("declaration policy changed inferred body demand", name)
		}
	}
	for _, name := range []string{"Borrowed", "ReadToken", "Fixed"} {
		if results[name].Status != "recommended" {
			t.Fatalf("%s: %+v", name, results[name])
		}
	}
	if results["Consume"].Status != "blocked" {
		t.Fatal(results["Consume"])
	}
	if results["Fixed"].Candidate != "ref int[100]" {
		t.Fatal(results["Fixed"])
	}
	var warned []string
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.LargeValueParameter {
			warned = append(warned, warning.Message)
			if strings.Contains(warning.Message, " or ref ") {
				t.Fatal("unverified view candidate", warning)
			}
		}
	}
	if len(warned) != 3 {
		t.Fatalf("unexpected advisories: %v", warned)
	}
	if !reflect.DeepEqual(before, analysis.Summaries()) {
		t.Fatal("contract policy mutated semantic demand")
	}
	read := parameterUsageSummaryNamed(t, analysis, "Read")
	if !read.Parameters[0].DeclaredType.CustomFree {
		t.Fatal("snapshot lost explicit lifecycle metadata")
	}
	read.Parameters[0].DeclaredType.CustomFree = false
	if !reflect.DeepEqual(before, analyzer.ParameterUsageAnalysis().Summaries()) {
		t.Fatal("snapshot edit changed analyzer-owned contracts")
	}
}

// TestParameterContractCleanupCarriers follows owned payloads and terminates
// recursive type walks while preserving reference and pointer boundaries.
// Rules: rules/memory/destruction.md — §§3.3(1), 3.4(2), 4(1–5).
func TestParameterContractCleanupCarriers(t *testing.T) {
	owner := Type{Name: "Owner", Kind: StructType, CustomFree: true}
	for _, typ := range []Type{
		{Name: "list", Kind: StructType, TypeArgs: []Type{owner}},
		{Name: "map", Kind: StructType, TypeArgs: []Type{{Kind: IntType}, owner}},
		{Name: "set", Kind: StructType, TypeArgs: []Type{owner}},
		{Kind: ResultType, TypeArgs: []Type{owner}},
		{Kind: UnionType, UnionVariants: []UnionVariant{{Payload: &owner}}},
		{Kind: UnionType, UnionVariants: []UnionVariant{{PayloadFields: []StructField{{Type: owner}}}}},
	} {
		if !parameterTypeHasCustomCleanup(typ, map[string]bool{}) {
			t.Fatal("lost owned cleanup", typ)
		}
	}
	for _, kind := range []TypeKind{ReferenceType, SliceType, RawPtrType, FunctionType} {
		if parameterTypeHasCustomCleanup(Type{Kind: kind, Element: &owner}, map[string]bool{}) {
			t.Fatal("borrowed cleanup", kind)
		}
	}
	recursive := Type{Name: "Recursive", Kind: StructType}
	recursive.Fields = []StructField{{Type: recursive}, {Type: owner}}
	if !parameterTypeHasCustomCleanup(recursive, map[string]bool{}) {
		t.Fatal("recursion hid later cleanup")
	}
	phantom := Type{Name: "Phantom", Kind: StructType, TypeArgs: []Type{owner}}
	if parameterTypeHasCustomCleanup(phantom, map[string]bool{}) {
		t.Fatal("generic argument alone does not prove ownership")
	}
}
