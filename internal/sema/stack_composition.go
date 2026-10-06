package sema

import (
	"errors"
	"sort"

	"sec/internal/lexer"
)

// ComposeSemanticStackSummaries composes producer-supplied own-frame facts for
// one exact plan key using the canonical reachable call graph. Missing frame
// facts remain Unknown. Results are detached and do not change the inputs.
// Producers must supply a graph covering the reported scope, including cleanup
// and runtime calls; this API does not establish call-graph completeness.
// Rules: rules/analysis/stack_analysis.md — "Call-path composition",
// "Control-flow composition", "Call graph ownership", "Acyclic call graphs",
// "Closed indirect-call target sets", and "CompilationPlan dependence".
func ComposeSemanticStackSummaries(graph *CallGraph, frames *StackSummaryStore, plan string) []SemanticStackSummary {
	return ComposeSemanticStackSummariesWithContracts(graph, frames, plan, nil)
}

// ComposeSemanticStackSummariesWithContracts additionally consumes verified open
// callable guarantees at the exact semantic plan key. Without a usable contract
// or supported reentry proof, calls remain Unknown and retain known evidence.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts",
// "CompilationPlan dependence", and "Reentry through callable and foreign boundaries".
func ComposeSemanticStackSummariesWithContracts(graph *CallGraph, frames *StackSummaryStore, plan string, contracts *StackCallableContractStore) []SemanticStackSummary {
	return ComposeSemanticStackSummariesWithBoundaryContracts(graph, frames, plan, StackBoundaryContracts{Callable: contracts})
}

// StackBoundaryContracts collects independently verified producer facts.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts" and "Foreign, runtime, and platform calls".
type StackBoundaryContracts struct {
	Callable *StackCallableContractStore
	External *StackExternalContractStore
}

// ComposeSemanticStackSummariesWithBoundaryContracts consumes callable and
// external guarantees at their exact plan key, without defining their imports.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls",
// "CompilationPlan dependence", and "Reentry through callable and foreign boundaries".
func ComposeSemanticStackSummariesWithBoundaryContracts(graph *CallGraph, frames *StackSummaryStore, plan string, contracts StackBoundaryContracts) []SemanticStackSummary {
	own := map[CallableID]StackBound{}
	for _, summary := range frames.SemanticSummaries() {
		if summary.CompilationPlanID == plan {
			own[summary.Callable] = summary.OwnFrame
		}
	}
	composed := composeKnownStack(graph, own, contracts.Callable, contracts.External, StackMeasurementSemantic, plan)
	var result StackSummaryStore
	for id, maximum := range composed {
		_ = result.RecordSemantic(SemanticStackSummary{Callable: id, CompilationPlanID: plan,
			OwnFrame: own[id], TransitiveMaximum: maximum.bound, MaximumCause: maximum.cause, Evidence: maximum.evidence})
	}
	return result.SemanticSummaries()
}

// ComposeMachineStackSummaries composes verified machine own-frame facts without
// using semantic estimates as machine evidence. Call-area/spill/layout costs
// must already belong to the supplied machine frame facts.
// The supplied graph must cover the reported scope, including cleanup/runtime
// calls, just as for ComposeSemanticStackSummaries.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement",
// "Call-path composition", "Closed indirect-call target sets", "CompilationPlan dependence", and "Machine-level revalidation".
func ComposeMachineStackSummaries(graph *CallGraph, frames *StackSummaryStore, plan string) ([]MachineStackSummary, error) {
	return ComposeMachineStackSummariesWithContracts(graph, frames, plan, nil)
}

// ComposeMachineStackSummariesWithContracts consumes independently verified
// machine contract/frame facts for one mandatory plan, never semantic estimates.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement",
// "Open callable contracts", "CompilationPlan dependence", and "Machine-level revalidation".
func ComposeMachineStackSummariesWithContracts(graph *CallGraph, frames *StackSummaryStore, plan string, contracts *StackCallableContractStore) ([]MachineStackSummary, error) {
	return ComposeMachineStackSummariesWithBoundaryContracts(graph, frames, plan, StackBoundaryContracts{Callable: contracts})
}

// ComposeMachineStackSummariesWithBoundaryContracts consumes machine guarantees
// independently of semantic estimates, for one mandatory CompilationPlan.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement",
// "Foreign, runtime, and platform calls", and "Machine-level revalidation".
func ComposeMachineStackSummariesWithBoundaryContracts(graph *CallGraph, frames *StackSummaryStore, plan string, contracts StackBoundaryContracts) ([]MachineStackSummary, error) {
	if plan == "" {
		return nil, errors.New("machine stack composition requires a CompilationPlan identity")
	}
	own := map[CallableID]StackBound{}
	for _, summary := range frames.MachineSummaries() {
		if summary.CompilationPlanID == plan {
			own[summary.Callable] = summary.OwnFrame
		}
	}
	composed := composeKnownStack(graph, own, contracts.Callable, contracts.External, StackMeasurementMachine, plan)
	var result StackSummaryStore
	for id, maximum := range composed {
		_ = result.RecordMachine(MachineStackSummary{Callable: id, CompilationPlanID: plan,
			OwnFrame: own[id], TransitiveMaximum: maximum.bound, MaximumCause: maximum.cause, Evidence: maximum.evidence})
	}
	return result.MachineSummaries(), nil
}

type stackCompositionValue struct {
	bound    StackBound
	cause    []StackCauseStep
	evidence StackEvidence
}

// composeKnownStack evaluates direct and closed-target acyclic dependencies bottom-up, reusing
// canonical same-stack SCCs rather than rediscovering recursion. Verified open
// callable and external contracts may contribute finite bounds; recursive,
// unsupported reentry and new-execution boundaries stay Unknown in this slice.
// Finite non-leaf results are upper bounds: a per-function maximum own frame
// need not coincide with the path that has the largest callee contribution.
// Rules: rules/analysis/stack_analysis.md — "Acyclic call graphs", "Recursion and SCCs",
// "Control-flow composition", "UpperBound", "Open calls without stack contracts",
// "Closed indirect-call target sets",
// "Open callable contracts", "Reentry through callable and foreign boundaries",
// "Foreign, runtime, and platform calls", "Error and panic paths", and "Physical stack domains".
// Legacy opaque invocations are represented by canonical unknown-callee effect
// sites rather than outgoing edges; they must not be mistaken for leaf bodies.
func composeKnownStack(graph *CallGraph, own map[CallableID]StackBound, contracts *StackCallableContractStore, external *StackExternalContractStore, level StackMeasurementLevel, plan string) map[CallableID]stackCompositionValue {
	results := map[CallableID]stackCompositionValue{}
	if graph == nil {
		return results
	}
	recursive := map[CallableID]bool{}
	components := map[CallableID][]CallableID{}
	for _, component := range graph.sameStackComponents() {
		members := make([]CallableID, 0, len(component))
		for member := range component {
			members = append(members, member)
		}
		sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })
		for id := range component {
			components[id] = members
			if len(component) > 1 {
				recursive[id] = true
			} else {
				for _, target := range graph.sameStackTargets(id) {
					if target == id {
						recursive[id] = true
					}
				}
			}
		}
	}
	var visit func(CallableID) stackCompositionValue
	visit = func(id CallableID) stackCompositionValue {
		if result, ok := results[id]; ok {
			return result
		}
		node, exists := graph.Node(id)
		result := stackCompositionValue{bound: own[id], cause: []StackCauseStep{{Callable: id, Source: node.Declaration, Detail: "own frame"}}}
		frame := StackFrameContribution{Callable: id, Source: node.Declaration, Frame: own[id]}
		if _, finite := own[id].Bytes(); finite {
			result.evidence.KnownPrefix = []StackFrameContribution{frame}
			result.evidence.Contributors = []StackFrameContribution{frame}
		}
		if own[id].Kind() == StackBoundUnknown {
			result.cause[0].Detail = "own-frame facts unavailable"
			result.evidence = mergeStackEvidence(result.evidence, stackBoundaryEvidence([]StackFrameContribution{frame}, result.cause[0]))
		}
		finish := func() stackCompositionValue { results[id] = result; return result }
		if !exists || node.Extern || recursive[id] {
			result.bound = UnknownStackBound()
			result.cause[0].Detail = "callable body or finite recursion depth unavailable"
			result.evidence = mergeStackEvidence(result.evidence, stackBoundaryEvidence([]StackFrameContribution{frame}, result.cause[0]))
			for _, member := range components[id] {
				if _, finite := own[member].Bytes(); finite {
					memberNode, _ := graph.Node(member)
					result.evidence = mergeStackEvidence(result.evidence, StackEvidence{Contributors: []StackFrameContribution{{Callable: member, Source: memberNode.Declaration, Frame: own[member]}}})
				}
			}
			return finish()
		}
		effects := cloneEffectSites(graph.effects[id])
		sort.Slice(effects, func(i, j int) bool {
			left, right := effects[i].Source, effects[j].Source
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Line != right.Line {
				return left.Line < right.Line
			}
			if left.Column != right.Column {
				return left.Column < right.Column
			}
			return effects[i].Kind < effects[j].Kind
		})
		sites := graph.Outgoing(id)
		for _, effect := range effects {
			if stackCallEffectCovered(graph, sites, effect, contracts, external, level, plan) {
				continue
			}
			if isPanicEffectKind(effect.Kind) {
				if runtime, usable := runtimeStackEffectContribution(id, effect, external, level, plan); usable {
					candidate := composeStackCall(own[id], runtime.bound)
					if stackCompositionStronger(candidate, result.bound) {
						result.bound = candidate
						result.cause = append([]StackCauseStep{{Callable: id, Source: node.Declaration, Detail: "own frame plus maximum runtime contribution"}}, runtime.cause...)
						result.evidence.KnownPrefix = []StackFrameContribution{frame}
					}
					continue
				}
				detail := "runtime panic-path stack contract unavailable"
				if effect.Kind == EffectMayPanicUnknownCallee {
					detail = "opaque callable stack contract unavailable"
				}
				boundary := StackCauseStep{Callable: id, Source: effect.Source, Detail: detail}
				if result.bound.Kind() != StackBoundUnknown && result.bound.Kind() != StackBoundUnbounded {
					result.cause = append(result.cause, boundary)
				}
				if result.bound.Kind() != StackBoundUnbounded {
					result.bound = UnknownStackBound()
				}
				result.evidence = mergeStackEvidence(result.evidence, stackBoundaryEvidence([]StackFrameContribution{frame}, boundary))
			}
		}
		for _, site := range sites {
			callee, externalBound := externalStackCallContribution(graph, site, external, level, plan)
			if !externalBound {
				if !site.TargetSet.IsClosed {
					callee = openStackCallContribution(graph, site, visit, contracts, level, plan)
				} else {
					callee = closedStackCallContribution(graph, site, visit)
				}
			}
			result.evidence = mergeStackEvidence(result.evidence, prependStackEvidence(frame, callee.evidence))
			candidate := composeStackCall(own[id], callee.bound)
			if stackCompositionStronger(candidate, result.bound) {
				result.bound = candidate
				result.cause = append([]StackCauseStep{{Callable: id, Source: node.Declaration, Detail: "own frame plus maximum callee contribution"}}, callee.cause...)
				result.evidence.KnownPrefix = append([]StackFrameContribution{frame}, callee.evidence.KnownPrefix...)
			}
		}
		return finish()
	}
	for _, node := range graph.Nodes() {
		visit(node.ID)
	}
	return results
}

// unknownStackCall retains the boundary source without inventing a zero-frame
// guarantee for foreign, indirect or runtime execution operations.
// Rules: rules/analysis/stack_analysis.md — "Unknown" and "Stack cause paths".
func unknownStackCall(source lexer.Token, detail string) stackCompositionValue {
	boundary := StackCauseStep{Source: source, Detail: detail}
	return stackCompositionValue{bound: UnknownStackBound(), cause: []StackCauseStep{boundary}, evidence: stackBoundaryEvidence(nil, boundary)}
}

// composeStackCall sums simultaneously live finite frames using arbitrary
// precision. Unknown or unbounded callee demand is conservatively Unknown here:
// this slice lacks the path/depth witness needed to propagate unboundedness.
// Rules: rules/analysis/stack_analysis.md — "Call-path composition", "UpperBound",
// "Unknown", and "Proven unbounded recursion".
func composeStackCall(own, callee StackBound) StackBound {
	left, leftFinite := own.Bytes()
	right, rightFinite := callee.Bytes()
	if !leftFinite || !rightFinite {
		return UnknownStackBound()
	}
	bound, _ := NewUpperStackBound(left.Add(left, right))
	return bound
}

// stackCompositionStronger selects the largest finite contribution or unknown
// boundary deterministically. Equal bounds prefer the first sorted call site;
// an existing proven unbounded own frame is never weakened by uncertainty.
// Rules: rules/analysis/stack_analysis.md — "Call-path composition", "Unknown",
// "Unbounded", "Stack cause paths", and "Determinism".
func stackCompositionStronger(candidate, current StackBound) bool {
	if current.Kind() == StackBoundUnbounded || current.Kind() == StackBoundUnknown {
		return false
	}
	if candidate.Kind() == StackBoundUnknown {
		return true
	}
	left, _ := candidate.Bytes()
	right, _ := current.Bytes()
	comparison := left.Cmp(right)
	return comparison > 0 || (comparison == 0 && candidate.Kind() == StackBoundUpperBound && current.Kind() == StackBoundExact)
}
