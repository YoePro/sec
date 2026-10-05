package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Exact extent follows reachable return materialization, while contiguity
// follows a resolved sequence Ptr operation. Declarations and weaker accesses
// alone do not establish either stronger requirement.
// Rules: rules/analysis/parameter_usage_analysis.md — "Exact extent and length observation are distinct",
// "Contiguous-sequence demand", "Raw pointer/address formation", "Calls propagate demand".
func TestParameterUsageExactExtentAndContiguousStorage(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_shape_demands_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, nil)
	analysis := analyzer.ParameterUsageAnalysis()
	demand := func(name string) ParameterDemand {
		return parameterUsageSummaryNamed(t, analysis, name).Parameters[0].Demand
	}
	for _, name := range []string{"ReturnArray", "ForwardArray", "ReturnField", "ReturnCarrier", "ReturnResult", "ReturnBorrow", "ReturnMoved"} {
		if !hasParameterShape(demand(name).Shapes, ParameterShapeExactExtent) {
			t.Errorf("%s lacks an exact return extent: %#v", name, demand(name))
		}
	}
	for _, name := range []string{"First", "Length", "ForwardRead", "Iterate"} {
		if hasParameterShape(demand(name).Shapes, ParameterShapeExactExtent) || hasParameterShape(demand(name).Shapes, ParameterShapeContiguousSequence) {
			t.Errorf("%s acquired an unproven strong shape: %#v", name, demand(name))
		}
	}
	for _, name := range []string{"ArrayPointer", "ForwardPointer", "DynamicPointer", "SlicePointer", "ListPointer", "StringPointer", "ProjectedPointer"} {
		d := demand(name)
		if !hasParameterShape(d.Shapes, ParameterShapeContiguousSequence) || d.Identity != ParameterAddressRequired || d.Precision != ParameterDemandExact {
			t.Errorf("%s pointer demand: %#v", name, d)
		}
		contiguous := false
		for _, storage := range d.Storage {
			if storage == ParameterStorageContiguous {
				contiguous = true
			}
			if storage == ParameterStorageNone {
				t.Errorf("%s retains no-special-storage beside contiguous demand", name)
			}
		}
		if !contiguous {
			t.Errorf("%s lacks contiguous storage demand", name)
		}
		if hasParameterShape(d.Shapes, ParameterShapeExactExtent) {
			t.Errorf("%s Ptr inferred an exact extent", name)
		}
	}
	scalar := demand("ScalarPointer")
	if scalar.Identity != ParameterAddressRequired || hasParameterShape(scalar.Shapes, ParameterShapeContiguousSequence) {
		t.Fatalf("scalar address was treated as sequence: %#v", scalar)
	}
	for _, use := range parameterUsageSummaryNamed(t, analysis, "ArrayPointer").Parameters[0].Uses {
		if use.Place.String() != "values" {
			t.Fatalf("pointer demand used a fictitious member Place: %s", use.Place.String())
		}
	}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.LargeValueParameter {
			t.Errorf("address-dependent parameter received narrowing advice: %v", warning)
		}
	}
}

// A Sema-proven dead return remains checked but contributes no shape demand.
// Rules: rules/analysis/parameter_usage_analysis.md — "Unreachable paths".
func TestParameterUsageExactExtentExcludesUnreachableReturn(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/parameter_shape_unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unreachable statement") {
		t.Fatalf("expected unreachable return diagnostic: %v", errors)
	}
	demand := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "DeadReturn").Parameters[0].Demand
	if hasParameterShape(demand.Shapes, ParameterShapeExactExtent) {
		t.Fatalf("dead return established exact extent: %#v", demand)
	}
}
