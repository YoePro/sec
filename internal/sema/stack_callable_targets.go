package sema

import "sort"

// closedStackCallContribution takes the maximum over every canonically
// permitted target of a same-stack closed call, including function values and
// capturing closures. Open contracts and incomplete or inconsistent body-to-
// callable mappings cannot provide a finite proof from their known subset.
// Sorting target identities makes equal-cost and unknown causes deterministic.
//
// Rules:
//   - rules/analysis/stack_analysis.md — "Closed indirect-call target sets", "Open callable contracts", "Open calls without stack contracts", "Stack cause paths", "Determinism"
//   - rules/analysis/call_graph.md — "Call-site record", "Dispatch kinds", "Execution relations"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets", "Callable body"
func closedStackCallContribution(graph *CallGraph, site CallSite, visit func(CallableID) stackCompositionValue) stackCompositionValue {
	if !sameStackExecution(site.Execution) || !site.TargetSet.IsClosed || site.TargetSet.HasOpenContract || site.TargetSet.OpenContract != "" ||
		len(site.TargetSet.KnownTargets) == 0 || len(site.Targets) != len(site.TargetSet.KnownTargets) {
		return unknownStackCall(site.Source, "complete closed callable target set unavailable")
	}
	switch site.Dispatch {
	case CallDispatchGenerated, CallDispatchDirect, CallDispatchStaticMethod, CallDispatchFunctionValue, CallDispatchClosure:
	default:
		return unknownStackCall(site.Source, "stack contract for foreign or execution-boundary call unavailable")
	}
	resolved := map[CallableID]bool{}
	for _, body := range site.TargetSet.KnownTargets {
		target, ok := graph.bodyNodes[body]
		if !ok || target == "" {
			return unknownStackCall(site.Source, "closed callable body has no resolved graph target")
		}
		resolved[target] = true
	}
	for _, target := range site.Targets {
		if !resolved[target] {
			return unknownStackCall(site.Source, "callable body and call-site target identities disagree")
		}
		delete(resolved, target)
	}
	if len(resolved) != 0 {
		return unknownStackCall(site.Source, "closed callable target coverage is incomplete")
	}
	targets := append([]CallableID(nil), site.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
	var maximum stackCompositionValue
	var evidence StackEvidence
	for index, target := range targets {
		candidate := visit(target)
		if candidate.bound.Kind() == StackBoundUnbounded {
			// rules/analysis/stack_analysis.md — "Proven unbounded recursion":
			// a may-target lacks the witness needed to propagate unboundedness.
			candidate.bound = UnknownStackBound()
			candidate.evidence = mergeStackEvidence(candidate.evidence, stackBoundaryEvidence(candidate.evidence.KnownPrefix, StackCauseStep{Callable: target, Source: site.Source, Detail: "unbounded may-target lacks a reachable-path witness"}))
		}
		evidence = mergeStackEvidence(evidence, candidate.evidence)
		if index == 0 || stackCompositionStronger(candidate.bound, maximum.bound) {
			maximum = candidate
		}
	}
	evidence.KnownPrefix = append([]StackFrameContribution(nil), maximum.evidence.KnownPrefix...)
	maximum.evidence = evidence
	maximum.cause = append([]StackCauseStep(nil), maximum.cause...)
	if len(maximum.cause) > 0 {
		maximum.cause[0].Source = site.Source
	}
	return maximum
}
