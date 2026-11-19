package main

import (
	"sec/internal/sema"
)

// analysisStackBound retains proof quality without converting missing evidence
// into a numeric zero. Bytes is decimal text to preserve arbitrary precision.
// Rules: rules/analysis/stack_analysis.md — §§6,58,75–76.
type analysisStackBound struct {
	Kind  string  `json:"kind"`
	Bytes *string `json:"bytes,omitempty"`
}

type analysisStackCause struct {
	Callable string         `json:"callable"`
	Location occurrenceSpan `json:"location"`
	Detail   string         `json:"detail"`
}

type analysisStackCallable struct {
	Callable            string               `json:"callable"`
	Name                string               `json:"name"`
	SemanticOwnFrame    analysisStackBound   `json:"semantic_own_frame"`
	SemanticMaximum     analysisStackBound   `json:"semantic_maximum"`
	FiniteSemanticProof bool                 `json:"finite_semantic_proof"`
	MachineAvailable    bool                 `json:"machine_available"`
	Cause               []analysisStackCause `json:"cause"`
}

// presentStackBound preserves the canonical proof quality and finite byte
// count without narrowing arbitrary precision or fabricating a missing size.
// Rules: rules/analysis/stack_analysis.md — §6; rules/tooling/cli.md — §6(6).
func presentStackBound(bound sema.StackBound) analysisStackBound {
	result := analysisStackBound{Kind: string(bound.Kind())}
	if size, known := bound.Bytes(); known {
		value := size.String()
		result.Bytes = &value
	}
	return result
}

// addStack consumes the shared Sema composer; it never estimates source frame
// sizes or derives machine frames from semantic facts. The currently missing
// own-frame producers remain Unknown. These optional facts do not create a
// budget diagnostic unless a canonical required-budget producer supplies one.
// Rules: rules/analysis/stack_analysis.md — §§6,58,75,76;
// rules/tooling/cli.md — §§5.1–5.4,6.
func (r *analyseReport) addStack(graph *sema.CallGraph, nodes []sema.CallableNode, frames *sema.StackSummaryStore, plan string) {
	selected := map[sema.CallableID]bool{}
	for _, node := range nodes {
		selected[node.ID] = true
	}
	section := r.section("stack")
	for _, summary := range sema.ComposeSemanticStackSummaries(graph, frames, plan) {
		if !selected[summary.Callable] {
			continue
		}
		node, ok := graph.Node(summary.Callable)
		if !ok {
			continue
		}
		entry := analysisStackCallable{Callable: string(summary.Callable), Name: analyseCallableName(node), SemanticOwnFrame: presentStackBound(summary.OwnFrame), SemanticMaximum: presentStackBound(summary.TransitiveMaximum), Cause: []analysisStackCause{}}
		entry.FiniteSemanticProof = entry.SemanticOwnFrame.Bytes != nil && entry.SemanticMaximum.Bytes != nil
		section.add("%s: semantic own=%s, maximum=%s; machine frame=not available", entry.Name, summary.OwnFrame.String(), summary.TransitiveMaximum.String())
		for _, cause := range summary.MaximumCause {
			location := occurrenceSpan{File: cause.Source.File, Start: occurrencePosition{Line: cause.Source.Line, Column: cause.Source.Column}, End: occurrencePosition{Line: cause.Source.Line, Column: cause.Source.Column}}
			entry.Cause = append(entry.Cause, analysisStackCause{Callable: string(cause.Callable), Location: location, Detail: cause.Detail})
			section.add("  %s at %s: %s", r.names[cause.Callable], analysePosition(cause.Source), cause.Detail)
		}
		r.stack = append(r.stack, entry)
	}
}
