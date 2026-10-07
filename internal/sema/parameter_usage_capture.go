package sema

import "sec/internal/ast"

// recordCaptureCreationDemand consumes validated capture transfer facts at
// closure creation, which executes in the enclosing callable. Lambda bodies
// execute in a separate callable and cannot contribute local demand by name.
// Capture/environment dependency demand remains partial and cannot authorize
// positive narrowing advice; separate lambda parameter summaries describe only
// the arguments of an invocation, not captured environment storage.
// Rules: rules/analysis/parameter_usage_analysis.md — "Inputs from other analyses",
// "Unknown critical dimensions block narrowing";
// rules/analysis/closure_analysis.md — "Capture record", "Callable creation".
func (b *parameterUsageBuilder) recordCaptureCreationDemand(lambda *ast.LambdaExpression) {
	captures, ok := b.analyzer.ResolvedLambdaCapturesOf(lambda)
	if !ok {
		return
	}
	for _, capture := range captures {
		id := b.analyzer.bindingIDs[sourceTokenLocation(capture.SourcePlace.RootToken)]
		parameter := b.byBinding[id]
		if parameter == nil {
			continue
		}
		if parameter.Demand.Access == ParameterAccessUnused {
			parameter.Demand.Access = ParameterAccessRead
		}
		kind := ParameterUseRead
		if capture.Transfer == CaptureTransferMove {
			kind = ParameterUseMove
			parameter.Demand.Ownership = strongerOwnership(parameter.Demand.Ownership, ParameterConsumptionRequired)
		}
		parameter.Demand.Precision = strongerPrecision(parameter.Demand.Precision, ParameterDemandPartial)
		parameter.Demand.Shapes = appendUniqueShape(parameter.Demand.Shapes, ParameterShapeWholeValue)
		parameter.Uses = append(parameter.Uses, ParameterUse{Kind: kind, Source: capture.Source, Place: cloneEscapePlace(capture.SourcePlace)})
	}
}
