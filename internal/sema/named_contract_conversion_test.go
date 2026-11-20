package sema

import "testing"

// The plan of a runtime conversion into a constrained named type lists the
// intrinsic carrier conversion and every contract in declaration order with
// its ContractKind and DeclarationIndex; the try channel is ConversionError;
// a carrier this plan cannot represent yet is named as unavailable.
// Rules: rules/types/contracts.md — §§ 12, 13.1; rules/mlir/packages/sec-mlir-dialect_package22.md — §§ 9–15.
func TestResolvedNamedContractConversionPlan(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Small int8 range 1..<10 odd
type Ratio float64 range 0.0..<1.0
type Pick float64 in [0.5, 1.5]

fn ToSmall(x: int) Result[Small, ConversionError] {
    let s := try Small(x)
    return Ok(s)
}

fn ToRatio(x: float64) Result[Ratio, ConversionError] {
    let r := try Ratio(x)
    return Ok(r)
}

fn ToPick(x: float64) Result[Pick, ConversionError] {
    let p := try Pick(x)
    return Ok(p)
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	plans := map[string]ResolvedNamedContractConversion{}
	for call := range analyzer.resolvedNamedContractConversions {
		plan, ok := analyzer.ResolvedNamedContractConversionOf(call)
		if !ok {
			t.Fatal("recorded plan is not published")
		}
		plans[callExpressionName(call)] = plan
	}
	small, ok := plans["Small"]
	if !ok || small.Unavailable != "" || small.ErrorType.Name != "ConversionError" || small.Intrinsic == nil || small.Intrinsic.Exact {
		t.Fatalf("Small plan = %+v", small)
	}
	if len(small.Predicates) != 2 || small.Predicates[0].Kind != "Range" || small.Predicates[0].DeclarationIndex != 0 ||
		!small.Predicates[0].Exclusive || small.Predicates[0].Min.Int64() != 1 || small.Predicates[0].Max.Int64() != 10 ||
		small.Predicates[1].Kind != "Odd" || small.Predicates[1].DeclarationIndex != 1 {
		t.Fatalf("Small predicates = %+v", small.Predicates)
	}
	small.Predicates[0].Min.SetInt64(99)
	for call := range analyzer.resolvedNamedContractConversions {
		if callExpressionName(call) != "Small" {
			continue
		}
		if fresh, _ := analyzer.ResolvedNamedContractConversionOf(call); fresh.Predicates[0].Min.Int64() != 1 {
			t.Fatal("a published plan shares state with the analyzer")
		}
	}
	ratio, ok := plans["Ratio"]
	if !ok || ratio.Unavailable != "" || len(ratio.Predicates) != 1 || ratio.Predicates[0].FormatBits != 64 ||
		ratio.Predicates[0].MinBits == nil || *ratio.Predicates[0].MinBits != 0 ||
		ratio.Predicates[0].MaxBits == nil || *ratio.Predicates[0].MaxBits != 0x3FF0000000000000 || !ratio.Predicates[0].MaxStrict {
		t.Fatalf("Ratio plan = %+v", ratio)
	}
	if pick, ok := plans["Pick"]; !ok || pick.Unavailable == "" {
		t.Fatalf("float membership must be named as unavailable: %+v", pick)
	}
}
