package sema

import "sort"

const parameterUsageProjectionLimit = 8

// propagateCalls joins direct and function-value demand in stable call-graph
// SCC order, then widens caller dimensions if the finite budget is exhausted.
// Rules: rules/analysis/parameter_usage_analysis.md — "Calls propagate demand",
// "Function-value calls", "Recursive functions";
// rules/compiler/compiler_analysis.md — §14(1–3).
func (b *parameterUsageBuilder) propagateCalls() {
	if len(b.callSites) == 0 {
		return
	}
	sites := b.orderedCallSites()
	limit := len(sites)*24 + len(b.result.summaries) + 1
	if configured := b.result.budget.MaxSummaryIterations; configured > 0 && configured < limit {
		limit = configured
	}
	if configured := b.analyzer.analysisBudget.MaxSummaryIterations; configured > 0 && configured < limit {
		limit = configured
	}
	result := runAnalysisFixedPoint(limit, func() bool {
		changed := false
		for index := range sites {
			if b.propagateCallSite(&sites[index]) {
				changed = true
			}
		}
		return changed
	}, func() {
		for index := range sites {
			for _, argument := range sites[index].arguments {
				widenParameterDemand(&argument.callerParameter.Demand)
			}
		}
	})
	b.result.iterations = result.Iterations
	b.result.converged = result.Converged
}

// orderedCallSites schedules joins by canonical call-graph SCC and source order.
// Rules: rules/analysis/parameter_usage_analysis.md — "Recursive functions"; rules/analysis/call_graph.md — "Recursion".
func (b *parameterUsageBuilder) orderedCallSites() []parameterUsageCallSite {
	sites := append([]parameterUsageCallSite(nil), b.callSites...)
	componentByCallable := map[CallableID]int{}
	if b.analyzer.callGraph != nil {
		for index, component := range b.analyzer.callGraph.sameStackComponents() {
			for id := range component {
				componentByCallable[id] = index
			}
		}
	}
	sort.SliceStable(sites, func(i, j int) bool {
		leftComponent, leftOK := componentByCallable[sites[i].caller]
		rightComponent, rightOK := componentByCallable[sites[j].caller]
		if leftOK != rightOK {
			return leftOK
		}
		if leftComponent != rightComponent {
			return leftComponent < rightComponent
		}
		left := sourceTokenLocation(sites[i].source)
		right := sourceTokenLocation(sites[j].source)
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	return sites
}

// propagateCallSite joins all known callee demands and projected evidence,
// including the public contract for omitted targets at an open boundary.
// Missing body summaries widen rather than proving absence of demand.
// Rules: rules/analysis/parameter_usage_analysis.md — "Calls propagate demand",
// "Function-value calls", "Dimension-specific unknown".
func (b *parameterUsageBuilder) propagateCallSite(site *parameterUsageCallSite) bool {
	changed := false
	for _, argument := range site.arguments {
		if site.open {
			changed = joinParameterDemand(&argument.callerParameter.Demand, openCallableParameterDemand(site.contract, argument.calleeIndex)) || changed
		}
		if len(site.targets) == 0 && !site.open {
			changed = widenParameterDemand(&argument.callerParameter.Demand) || changed
		}
		for _, id := range site.targets {
			target := b.result.summaries[id]
			var callee *ParameterUsageParameterSummary
			if target != nil {
				if argument.receiver {
					callee = target.Receiver
				} else if argument.calleeIndex >= 0 && argument.calleeIndex < len(target.Parameters) {
					callee = &target.Parameters[argument.calleeIndex]
				}
			}
			if callee == nil {
				changed = widenParameterDemand(&argument.callerParameter.Demand) || changed
				continue
			}
			changed = joinParameterDemand(&argument.callerParameter.Demand, callee.Demand) || changed
			for _, use := range callee.Uses {
				place, widened := instantiateParameterUsePlace(argument.callerPlace, use.Place)
				if widened {
					changed = setDemandPrecision(&argument.callerParameter.Demand, ParameterDemandPartial) || changed
				}
				propagated := ParameterUse{Kind: ParameterUseCall, Source: site.source, Place: place}
				if appendUniqueParameterUse(argument.callerParameter, propagated) {
					changed = true
				}
			}
		}
	}
	return changed
}

// instantiateParameterUsePlace translates callee projections onto caller storage
// and marks precision loss when recursive projection growth reaches its bound.
// Rules: rules/analysis/parameter_usage_analysis.md — "Calls propagate demand", "Recursive functions"; rules/compiler/compiler_analysis.md — §14(2–3).
func instantiateParameterUsePlace(base Place, callee Place) (Place, bool) {
	result := cloneEscapePlace(base)
	widened := false
	for _, projection := range callee.Projections {
		if len(result.Projections) >= parameterUsageProjectionLimit {
			widened = true
			break
		}
		result = appendPlaceProjection(result, projection)
	}
	return result, widened
}

// appendUniqueParameterUse retains each semantic call-site/place witness once,
// permitting a finite evidence fixed point across recursion.
// Rules: rules/compiler/compiler_analysis.md — §14(2); rules/analysis/parameter_usage_analysis.md — "Recursive functions".
func appendUniqueParameterUse(parameter *ParameterUsageParameterSummary, candidate ParameterUse) bool {
	for _, existing := range parameter.Uses {
		if existing.Kind == candidate.Kind && sameSourceToken(existing.Source, candidate.Source) && existing.Place.String() == candidate.Place.String() {
			return false
		}
	}
	parameter.Uses = append(parameter.Uses, candidate)
	return true
}

// joinParameterDemand monotonically raises each independent demand dimension.
// Rules: rules/analysis/parameter_usage_analysis.md — "Calls propagate demand", "Control-flow joins".
func joinParameterDemand(target *ParameterDemand, source ParameterDemand) bool {
	changed := false
	changed = setAccessDemand(target, strongerAccess(target.Access, source.Access)) || changed
	changed = setMutationDemand(target, strongerMutation(target.Mutation, source.Mutation)) || changed
	changed = setOwnershipDemand(target, strongerOwnership(target.Ownership, source.Ownership)) || changed
	changed = setLifetimeDemand(target, strongerLifetime(target.Lifetime, source.Lifetime)) || changed
	changed = setIdentityDemand(target, strongerIdentity(target.Identity, source.Identity)) || changed
	for _, shape := range source.Shapes {
		before := len(target.Shapes)
		target.Shapes = appendUniqueShape(target.Shapes, shape)
		changed = len(target.Shapes) != before || changed
	}
	if source.MinimumExtent > target.MinimumExtent {
		target.MinimumExtent = source.MinimumExtent
		changed = true
	}
	for _, storage := range source.Storage {
		if storage == ParameterStorageNone && hasSpecialParameterStorage(target.Storage) {
			continue
		}
		if storage != ParameterStorageNone {
			target.Storage = removeParameterStorage(target.Storage, ParameterStorageNone)
		}
		before := len(target.Storage)
		target.Storage = appendUniqueParameterStorage(target.Storage, storage)
		changed = len(target.Storage) != before || changed
	}
	changed = setRepresentationDemand(target, strongerRepresentation(target.Representation, source.Representation)) || changed
	changed = setDemandPrecision(target, strongerPrecision(target.Precision, source.Precision)) || changed
	return changed
}

// widenParameterDemand publishes Unknown critical capabilities when inference
// cannot finish; incomplete summaries must never justify API narrowing.
// Rules: rules/analysis/parameter_usage_analysis.md — "Dimension-specific unknown"; rules/compiler/compiler_analysis.md — §14(3), §15(3–5).
func widenParameterDemand(demand *ParameterDemand) bool {
	before := cloneParameterDemand(*demand)
	demand.Access = ParameterAccessUnknown
	demand.Mutation = ParameterUnknownMutation
	demand.Ownership = ParameterUnknownOwnership
	demand.Lifetime = ParameterLifetimeUnknown
	demand.Identity = ParameterUnknownIdentity
	demand.Shapes = appendUniqueShape(demand.Shapes, ParameterShapeUnknown)
	demand.Storage = removeParameterStorage(demand.Storage, ParameterStorageNone)
	demand.Storage = appendUniqueParameterStorage(demand.Storage, ParameterStorageUnknown)
	demand.Representation = ParameterRepresentationUnknown
	demand.Precision = ParameterDemandUnknown
	return !parameterDemandsEqual(before, *demand)
}
