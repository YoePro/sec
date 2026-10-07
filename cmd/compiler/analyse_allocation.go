package main

import "sec/internal/sema"

// addAllocationFacts presents the same compiler-produced knowledge as LSP;
// unknown effects are analysis facts, never fabricated allocation diagnoses.
// Selection remains source-qualified and does not hide transitive evidence.
// Rules: rules/memory/allocation.md — §§24(4),(6),29(1)-(3);
// rules/compiler/compiler_analysis.md — §61.
func (r *analyseReport) addAllocationFacts(analyzer *sema.Analyzer, nodes []sema.CallableNode, names map[sema.CallableID]string) {
	section := r.section("allocation")
	facts := map[sema.CallableID]sema.AllocationCallableFact{}
	for _, fact := range analyzer.AllocationFacts() {
		facts[fact.Callable] = fact
	}
	for _, node := range nodes {
		fact := facts[node.ID]
		section.add("%s: allocation %s", names[node.ID], fact.Knowledge)
		if len(fact.AllocationPath) > 0 {
			section.add("  allocation path: %s", analysePath(fact.AllocationPath, names))
		}
		for _, context := range analyzer.AllocationContexts(node.ID) {
			section.add("  %s at %s via %s", sema.AllocationContextDescription(context), analysePosition(context.Source), analysePath(context.Path, names))
		}
		if fact.HasUnknown {
			section.add("  unresolved allocation behavior")
			if len(fact.UnknownPath) > 0 {
				section.add("  unknown allocation path: %s", analysePath(fact.UnknownPath, names))
			}
			for _, evidence := range fact.UnknownEvidence {
				section.add("  unknown: %s at %s", evidence.Reason, analysePosition(evidence.Source))
			}
		}
	}
}
