package sema

import (
	"fmt"
	"math/big"
	"testing"

	"sec/internal/ast"
)

// Parameter demand must use the same exact, composed ranges as the borrow
// solver, including CTE endpoints and conservative dynamic parents.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§24–30;
// rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses".
func TestParameterSlicePlacesUseCanonicalRanges(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			program, analyzer, errs := readSliceMatrixInput(t, "parameter_slice_places_valid.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			// Conflicting post-analysis constants must not reinterpret runtime
			// endpoints in an unrelated callable's environment.
			analyzer.constInts = map[string]*big.Int{"start": big.NewInt(0), "finish": big.NewInt(8)}
			ranges, indexes := len(analyzer.placeIdentities.sliceRanges), len(analyzer.placeIdentities.dynamicIndexes)
			expected := map[string]string{"Full": "values[..<8]", "Nested": "values[3..<7]", "Indexed": "values[3]", "Computed": "values[2..<5]", "Dynamic": "values[*..][1..<2]", "Open": "values[1..]"}
			for _, statement := range program.Statements {
				function, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				summary := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), function.Name.Value)
				parameter := summary.Parameters[0]
				builder := parameterUsageBuilder{analyzer: analyzer, byBinding: map[BindingID]*ParameterUsageParameterSummary{parameter.Binding: &parameter}}
				expression := function.Body.Statements[0].(*ast.ReturnStatement).Value
				_, place, resolved := builder.parameterPlace(expression)
				if !resolved || place.String() != expected[function.Name.Value] {
					t.Fatalf("%s: resolved=%v place=%s, want %s", function.Name.Value, resolved, place.String(), expected[function.Name.Value])
				}
				if function.Name.Value == "Dynamic" && (len(place.Projections) != 2 || place.Projections[0].DynamicRange == 0) {
					t.Fatal("lost dynamic parent identity", place)
				}
				if function.Name.Value == "Open" && !place.Projections[0].SliceEndOpen {
					t.Fatal("lost omitted-end relationship", place)
				}
				// Projection results may be inspected or edited by a consumer
				// without mutating the retained analysis facts.
				for _, projection := range place.Projections {
					for _, value := range []*big.Int{projection.SliceStart, projection.SliceEnd, projection.ConstantIndex} {
						if value != nil {
							value.SetInt64(99)
						}
					}
				}
				_, again, _ := builder.parameterPlace(expression)
				if again.String() != expected[function.Name.Value] {
					t.Fatal("consumer mutated retained projections", again)
				}
			}
			if ranges != len(analyzer.placeIdentities.sliceRanges) || indexes != len(analyzer.placeIdentities.dynamicIndexes) {
				t.Fatal("parameter consumer allocated evaluation identities")
			}
		})
	}
}
