package main

import "sec/internal/sema"

const isrInspectionLimitation = "ISR analysis is incomplete: resolved interrupt roots and profiles, blocking, stack, race, deadlock, runtime and foreign-safety requirements are not fully verified; --verbose shows existing callable panic/allocation prerequisites, not ISR safety proof"

// addISRPrerequisites presents only existing canonical summary facts for the
// selected callable set. Nodes are not inferred interrupt roots, and neither
// absent panic/allocation evidence nor an empty selection establishes safety.
// Missing profile/proof producers remain an explicit limitation at every depth.
// Rules: rules/analysis/isr_analysis.md — §§1–2,32–33;
// rules/tooling/cli.md — §§5.3–5.6,6.
func (r *analyseReport) addISRPrerequisites(graph *sema.CallGraph, nodes []sema.CallableNode, names map[sema.CallableID]string) {
	r.limitation("%s", isrInspectionLimitation)
	section := r.section("ISR prerequisites (not a safety proof)")
	for _, node := range nodes {
		panicFacts := graph.EffectSummary(node.ID)
		allocation := graph.ArenaSummary(node.ID)
		section.add("%s: callable summary may-panic=%t, may-allocate=%t, allocation-unknown=%t at %s", names[node.ID], panicFacts.MayPanic, allocation.MayAllocate, allocation.AllocationUnknown, analysePosition(node.Declaration))
	}
	r.addEffects(graph, nodes, names)
}
