package main

import (
	"sec/internal/sema"
	"strings"
)

// allocationHoverLines renders the compiler's allocation knowledge without
// treating unknown behavior as allocation or an empty summary as a free proof.
// Positive and unknown paths stay independently visible for mixed bodies.
// Rules: rules/memory/allocation.md — §§24(2),(6),29(1)-(3);
// rules/tooling/lsp.md — "Hover".
func allocationHoverLines(analyzer *sema.Analyzer, graph *sema.CallGraph, id sema.CallableID) []string {
	fact := analyzer.AllocationFact(id)
	lines := []string{"Allocation behavior: `" + string(fact.Knowledge) + "`"}
	renderPath := func(path []sema.CallableID) string {
		names := make([]string, 0, len(path))
		for _, id := range path {
			if node, known := graph.Node(id); known {
				names = append(names, node.Name)
			}
		}
		return "`" + strings.Join(names, "` -> `") + "`"
	}
	if fact.Knowledge == sema.AllocationMayAllocate {
		lines = append(lines, "May allocate: `yes`", "Allocation path: "+renderPath(fact.AllocationPath))
	}
	for _, context := range analyzer.AllocationContexts(id) {
		lines = append(lines, sema.AllocationContextDescription(context)+" via "+renderPath(context.Path))
	}
	if fact.HasUnknown {
		lines = append(lines, "Unresolved allocation behavior: `yes`")
		if len(fact.UnknownPath) > 0 {
			lines = append(lines, "Unknown allocation path: "+renderPath(fact.UnknownPath))
		}
		for _, evidence := range fact.UnknownEvidence {
			lines = append(lines, evidence.Reason)
		}
	}
	return lines
}
