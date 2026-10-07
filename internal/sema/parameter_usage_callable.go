package sema

import (
	"sort"

	"sec/internal/ast"
)

type parameterUsageInvocation struct {
	caller CallableID
	source sourceTokenKey
}

// walkFunctionValueCall consumes the invocation's canonical may-target set;
// it never rediscovers a body from a function name or the last assignment.
// Rules: rules/analysis/parameter_usage_analysis.md — "Function-value calls",
// "Calls propagate demand"; rules/analysis/closure_analysis.md — "Soundness of target sets".
func (b *parameterUsageBuilder) walkFunctionValueCall(call *ast.CallExpression) bool {
	if b.functionValueCalls == nil {
		b.functionValueCalls = map[parameterUsageInvocation]CallSite{}
		if b.analyzer.callGraph != nil {
			for _, site := range b.analyzer.callGraph.sites {
				if site.Dispatch == CallDispatchFunctionValue || site.Dispatch == CallDispatchClosure {
					b.functionValueCalls[parameterUsageInvocation{site.Caller, sourceTokenLocation(site.Source)}] = site
				}
			}
		}
	}
	graphSite, ok := b.functionValueCalls[parameterUsageInvocation{b.summary.Callable, sourceTokenLocation(call.Token)}]
	if !ok {
		return false
	}
	capability := CallableShared
	contract := cloneOpenCallableContract(graphSite.TargetSet.Contract)
	if contract != nil {
		capability = normalizedCallableCapability(contract.Capability)
	}
	if typ, known := b.analyzer.ResolvedTypeOf(call.Callee); known {
		capability = normalizedCallableCapability(typ.FunctionCapability)
		if contract == nil {
			contract = functionTypeGraphContract(typ)
		}
	}
	ownership := ParameterBorrowSufficient
	identity := ParameterValueOnly
	if capability == CallableMutable {
		identity = ParameterAddressRequired
	}
	if capability == CallableConsuming {
		ownership = ParameterConsumptionRequired
	}
	b.markExpression(call.Callee, ParameterUseCall, capability == CallableMutable, ownership, identity)
	b.walkExpressionChildren(call.Callee)
	site := parameterUsageCallSite{caller: b.summary.Callable, source: call.Token,
		targets: append([]CallableID(nil), graphSite.Targets...), open: !graphSite.TargetSet.IsClosed,
		contract: contract}
	for index, argument := range call.Arguments {
		argumentSource := parameterUsageTransferSource(argument)
		// Reference formation and explicit moves execute in the caller, even
		// when the callee never reads the argument. Body demand is joined later.
		b.walkExpression(argument)
		if contract != nil && index < len(contract.Parameters) && contract.Parameters[index].Ref {
			b.markExpression(argumentSource, ParameterUseCall, contract.Parameters[index].MutableRef, ParameterBorrowSufficient, ParameterAddressRequired)
		}
		if prefix, moved := argument.(*ast.PrefixExpression); moved && prefix.Operator == "<-" {
			b.markExpression(argumentSource, ParameterUseMove, false, ParameterConsumptionRequired, ParameterValueOnly)
		}
		if parameter, place, rooted := b.parameterPlace(argumentSource); rooted {
			site.arguments = append(site.arguments, parameterUsageCallArgument{callerParameter: parameter,
				callerPlace: cloneEscapePlace(place), calleeIndex: index})
		}
	}
	if len(site.arguments) != 0 {
		b.callSites = append(b.callSites, site)
	}
	return true
}

// openCallableParameterDemand retains signature-proven invocation capabilities
// independently of absent retention, shape, storage and representation promises.
// An ordinary source fn type carries none of those optional guarantees.
// Rules: rules/analysis/parameter_usage_analysis.md — "Function-value calls",
// "Dimension-specific unknown", "Unknown critical dimensions block narrowing";
// rules/corrections/applied/types-callable-model-correction-20260816.md — callable parameter modes.
func openCallableParameterDemand(contract *OpenCallableContract, index int) ParameterDemand {
	demand := defaultParameterDemand()
	widenParameterDemand(&demand)
	if contract == nil || index < 0 || len(contract.Parameters) == 0 {
		return demand
	}
	if index >= len(contract.Parameters) {
		index = len(contract.Parameters) - 1
		if !contract.Parameters[index].Variadic {
			return demand
		}
	}
	parameter := contract.Parameters[index]
	if parameter.Ref {
		demand.Identity = ParameterAddressRequired
		demand.Ownership = ParameterBorrowSufficient
		if !parameter.MutableRef {
			demand.Access = ParameterAccessRead
			demand.Mutation = ParameterNoMutation
		}
	} else {
		demand.Shapes = appendUniqueShape(demand.Shapes, ParameterShapeWholeValue)
	}
	if parameter.Consuming {
		demand.Ownership = ParameterConsumptionRequired
	}
	demand.Precision = ParameterDemandPartial
	return demand
}

// analyzeLambdaParameters builds body-local summaries under the closure
// producer's canonical callable IDs. Captures are not lambda arguments and
// remain owned by creation/environment analysis. Missing lambda escape summaries
// leave aggregate/callable ownership and retention unknown, never call-local proof.
// Rules: rules/analysis/parameter_usage_analysis.md — "Function-value calls",
// "Inputs from other analyses", "Dimension-specific unknown";
// rules/analysis/closure_analysis.md — "Callable body", "Callable creation".
func (b *parameterUsageBuilder) analyzeLambdaParameters() {
	type item struct {
		lambda *ast.LambdaExpression
		id     CallableID
	}
	var lambdas []item
	for expression, identity := range b.analyzer.resolvedCallableIdentities {
		lambda, ok := expression.(*ast.LambdaExpression)
		if !ok || lambda.Body == nil || b.analyzer.callGraph == nil {
			continue
		}
		if id, known := b.analyzer.callGraph.bodyNodes[identity.Body]; known {
			lambdas = append(lambdas, item{lambda, id})
		}
	}
	sort.Slice(lambdas, func(i, j int) bool { return lambdas[i].id < lambdas[j].id })
	for _, item := range lambdas {
		summary := &ParameterUsageCallableSummary{Callable: item.id, Name: "lambda", Declaration: item.lambda.Token, Precision: ParameterDemandExact}
		b.summary = summary
		b.byBinding = map[BindingID]*ParameterUsageParameterSummary{}
		b.byName = map[string]*ParameterUsageParameterSummary{}
		for index, parameter := range item.lambda.Parameters {
			fact := b.parameterFact(parameter)
			demand := defaultParameterDemand()
			if fact.ID == 0 || fact.Type.Kind == InvalidType {
				widenParameterDemand(&demand)
			}
			if typeContainsReference(fact.Type, map[string]bool{}) {
				demand.Lifetime = ParameterLifetimeUnknown
				demand.Precision = ParameterDemandPartial
			}
			// Unlike scalar copies, owned carriers can preserve dependencies
			// through aliases, returns or longer-lived destinations. Their
			// missing escape summary must not justify borrowing/narrowing.
			switch fact.Type.Kind {
			case BoolType, CharType, RuneType, IntType, UintType, FloatType, DecimalType, EnumType, RegisterType, ReferenceType:
			default:
				demand.Ownership = ParameterUnknownOwnership
				demand.Lifetime = ParameterLifetimeUnknown
				demand.Identity = ParameterUnknownIdentity
				demand.Precision = ParameterDemandPartial
			}
			summary.Parameters = append(summary.Parameters, ParameterUsageParameterSummary{Binding: fact.ID, Index: index,
				Name: parameter.Name.Value, Declaration: parameter.Name.Token, DeclaredType: semanticSnapshotType(fact.Type),
				DeclaredRef: parameter.Ref, DeclaredMut: parameter.MutableRef, Consuming: parameter.Consuming, Demand: demand})
		}
		for index := range summary.Parameters {
			parameter := &summary.Parameters[index]
			if parameter.Binding != 0 {
				b.byBinding[parameter.Binding] = parameter
			}
			b.byName[parameter.Name] = parameter
		}
		b.walkBlock(item.lambda.Body)
		// The legacy escape producer excludes lambda returns and still uses
		// enclosing function parameter metadata for other lambda events. Its
		// records are not a body-local lambda parameter contract.
		b.result.summaries[item.id] = summary
		b.result.summaryOrder = append(b.result.summaryOrder, item.id)
	}
}
