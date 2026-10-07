package main

import (
	"fmt"
	"sec/internal/sema"
	"strings"
)

// callGraphHoverSuffix consumes canonical callable relations and effects for
// the one compiler-resolved declaration at the hover position.
// Rules: rules/analysis/call_graph.md — "One canonical graph, multiple analysis views";
// rules/tooling/lsp.md — "Hover"; rules/memory/allocation.md — §29.
func callGraphHoverSuffix(analyzer *sema.Analyzer, uri string, text string, pos position) string {
	if analyzer == nil {
		return ""
	}
	token, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return ""
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
	if len(definitions) != 1 {
		return ""
	}
	graph := analyzer.CallGraph()
	nodes := graph.NodesForDeclaration(definitions[0])
	if len(nodes) != 1 {
		return ""
	}
	node := nodes[0]
	incoming := graph.Incoming(node.ID)
	outgoing := graph.Outgoing(node.ID)
	callerCount := distinctCallers(incoming)
	calleeCount := distinctCallees(outgoing)

	lines := []string{
		"**Call graph**",
		fmt.Sprintf("Incoming: `%d` call sites from `%d` callables", len(incoming), callerCount),
		fmt.Sprintf("Outgoing: `%d` call sites to `%d` callables", len(outgoing), calleeCount),
	}
	roots := graph.RootsReaching(node.ID)
	if len(roots) == 0 {
		lines = append(lines, "Reachability: no active root in the current analysis")
	} else {
		rootNames := make([]string, 0, len(roots))
		for _, root := range roots {
			rootNames = append(rootNames, string(root.Kind))
		}
		lines = append(lines, "Reachable from: `"+strings.Join(rootNames, "`, `")+"`")
	}
	if graph.IsSameStackRecursive(node.ID) {
		members := graph.SameStackSCC(node.ID)
		lines = append(lines, callGraphComponentHoverLine("Same-stack recursion", members))
	}
	if graph.IsInTaskSpawnCycle(node.ID) {
		lines = append(lines, callGraphComponentHoverLine("Task-spawn cycle", graph.TaskSpawnSCC(node.ID)))
	}
	if graph.IsInThreadStartCycle(node.ID) {
		lines = append(lines, callGraphComponentHoverLine("Thread-start cycle", graph.ThreadStartSCC(node.ID)))
	}
	if graph.IsInProcessLaunchCycle(node.ID) {
		lines = append(lines, callGraphComponentHoverLine("Process-launch cycle", graph.ProcessLaunchSCC(node.ID)))
	}
	spawnCounts := map[sema.CallExecutionRelation]int{}
	for _, site := range outgoing {
		switch site.Execution {
		case sema.CallExecutionSpawnTask, sema.CallExecutionSpawnThread, sema.CallExecutionSpawnProcess:
			spawnCounts[site.Execution]++
		}
	}
	if len(spawnCounts) > 0 {
		parts := make([]string, 0, len(spawnCounts))
		for _, execution := range []sema.CallExecutionRelation{
			sema.CallExecutionSpawnTask,
			sema.CallExecutionSpawnThread,
			sema.CallExecutionSpawnProcess,
		} {
			if count := spawnCounts[execution]; count > 0 {
				parts = append(parts, fmt.Sprintf("%s: `%d`", strings.ReplaceAll(string(execution), "-", " "), count))
			}
		}
		lines = append(lines, "Execution edges: "+strings.Join(parts, ", "))
	}
	arenaSummary := graph.ArenaSummary(node.ID)
	if len(arenaSummary.DirectEffects) > 0 {
		effects := make([]string, 0, len(arenaSummary.DirectEffects))
		for _, effect := range arenaSummary.DirectEffects {
			name := string(effect.Kind)
			if effect.Arena != "" {
				name += "(" + effect.Arena + ")"
			}
			effects = append(effects, name)
		}
		lines = append(lines, "Direct Arena effects: `"+strings.Join(effects, "`, `")+"`")
	}
	lines = append(lines, allocationHoverLines(analyzer, graph, node.ID)...)
	lines = append(lines, panicHoverLines(graph, node.ID)...)
	lines = append(lines, blockingHoverLine(graph, node.ID))
	return "\n\n" + strings.Join(lines, "\n\n")
}
