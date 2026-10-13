package main

import (
	"fmt"
	"sec/internal/sema"
	"strings"
)

// callHierarchyItemsForSource prepares compiler-resolved declarations with canonical effect and assignment flow.
// Rules: rules/tooling/lsp.md — "Call hierarchy"; rules/errors/errorhandling.md — §36.
func callHierarchyItemsForSource(uri string, text string, pos position, overlays ...sourceOverlay) (items []callHierarchyItem) {
	defer func() {
		if recover() != nil {
			items = []callHierarchyItem{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text, overlays...)
	if analyzer == nil {
		return []callHierarchyItem{}
	}
	token, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return []callHierarchyItem{}
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
	if len(definitions) != 1 {
		return []callHierarchyItem{}
	}
	overlay := firstSourceOverlay(overlays)
	for _, node := range analyzer.CallGraph().NodesForDeclaration(definitions[0]) {
		items = append(items, withTryAssignmentDetail(withPanicDetail(callHierarchyItemForNode(uri, text, overlay, node), analyzer.CallGraph(), node.ID), analyzer, node.ID))
	}
	return items
}

// callHierarchyIncomingCallsForSource groups canonical incoming sites and presents each caller's own facts.
// Rules: rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"; rules/errors/errorhandling.md — §36.
func callHierarchyIncomingCallsForSource(uri string, text string, nodeID sema.CallableID, overlays ...sourceOverlay) (calls []callHierarchyIncomingCall) {
	defer func() {
		if recover() != nil {
			calls = []callHierarchyIncomingCall{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text, overlays...)
	if analyzer == nil {
		return []callHierarchyIncomingCall{}
	}
	graph := analyzer.CallGraph()
	overlay := firstSourceOverlay(overlays)
	grouped := map[string]int{}
	for _, site := range graph.Incoming(nodeID) {
		caller, ok := graph.Node(site.Caller)
		if !ok {
			continue
		}
		rng := definitionTokenRange(sourceTextForToken(uri, text, overlay, site.Source), site.Source)
		key := callHierarchyRelationshipKey(caller.ID, site.Dispatch, site.Execution)
		if index, exists := grouped[key]; exists {
			calls[index].FromRanges = append(calls[index].FromRanges, rng)
			continue
		}
		grouped[key] = len(calls)
		calls = append(calls, callHierarchyIncomingCall{
			From:       withTryAssignmentDetail(withPanicDetail(callHierarchyItemForRelationship(uri, text, overlay, caller, site.Dispatch, site.Execution), graph, caller.ID), analyzer, caller.ID),
			FromRanges: []lspRange{rng},
		})
	}
	return calls
}

// callHierarchyOutgoingCallsForSource groups canonical targets and presents each callee's own facts.
// Rules: rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"; rules/errors/errorhandling.md — §36.
func callHierarchyOutgoingCallsForSource(uri string, text string, nodeID sema.CallableID, overlays ...sourceOverlay) (calls []callHierarchyOutgoingCall) {
	defer func() {
		if recover() != nil {
			calls = []callHierarchyOutgoingCall{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text, overlays...)
	if analyzer == nil {
		return []callHierarchyOutgoingCall{}
	}
	graph := analyzer.CallGraph()
	overlay := firstSourceOverlay(overlays)
	grouped := map[string]int{}
	for _, site := range graph.Outgoing(nodeID) {
		for _, targetID := range site.Targets {
			target, ok := graph.Node(targetID)
			if !ok || target.Declaration.Line <= 0 {
				continue
			}
			rng := definitionTokenRange(sourceTextForToken(uri, text, overlay, site.Source), site.Source)
			key := callHierarchyRelationshipKey(target.ID, site.Dispatch, site.Execution)
			if index, exists := grouped[key]; exists {
				calls[index].FromRanges = append(calls[index].FromRanges, rng)
				continue
			}
			grouped[key] = len(calls)
			calls = append(calls, callHierarchyOutgoingCall{
				To:         withTryAssignmentDetail(withPanicDetail(callHierarchyItemForRelationship(uri, text, overlay, target, site.Dispatch, site.Execution), graph, target.ID), analyzer, target.ID),
				FromRanges: []lspRange{rng},
			})
		}
	}
	return calls
}

// withPanicDetail marks call hierarchy items whose callable may panic, so
// transitive panic-capable paths are visible while navigating the hierarchy.
//
// Rules:
//   - rules/errors/panic.md — § 28 "Diagnostics and tooling"
//   - rules/tooling/lsp.md — call hierarchy effect facts
func withPanicDetail(item callHierarchyItem, graph *sema.CallGraph, id sema.CallableID) callHierarchyItem {
	if graph != nil && graph.EffectSummary(id).MayPanic {
		if item.Detail != "" {
			item.Detail += " · "
		}
		item.Detail += "may panic"
	}
	return item
}

func callHierarchyRelationshipKey(node sema.CallableID, dispatch sema.CallDispatchKind, execution sema.CallExecutionRelation) string {
	return fmt.Sprintf("%s|%s|%s", node, dispatch, execution)
}

func callHierarchyItemForRelationship(analysisURI string, analysisText string, overlay sourceOverlay, node sema.CallableNode, dispatch sema.CallDispatchKind, execution sema.CallExecutionRelation) callHierarchyItem {
	item := callHierarchyItemForNode(analysisURI, analysisText, overlay, node)
	item.Data.Dispatch = dispatch
	item.Data.Execution = execution
	if execution != "" && execution != sema.CallExecutionSynchronous {
		label := strings.ReplaceAll(string(execution), "-", " ")
		if item.Detail == "" {
			item.Detail = label
		} else {
			item.Detail += " | " + label
		}
	}
	return item
}

// analyzeNavigationSource runs the shared frontend on the immutable editor source view.
// Rules: rules/tooling/lsp.md — "Snapshots", "Project and source integration".
func analyzeNavigationSource(uri string, text string, overlays ...sourceOverlay) *sema.Analyzer {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	program, analyzer, release := cachedLSPAnalysis(uri, text, program, firstSourceOverlay(overlays))
	defer release()
	return analyzer
}

// callHierarchyItemForNode uses the compiler declaration identity and occurrence for navigation.
// Rules: rules/tooling/lsp.md — "Call hierarchy"; rules/analysis/call_graph.md — "Graph data model".
func callHierarchyItemForNode(analysisURI string, analysisText string, overlay sourceOverlay, node sema.CallableNode) callHierarchyItem {
	uri := analysisURI
	if node.Declaration.File != "" {
		uri = uriFromPath(node.Declaration.File)
	}
	rng := definitionTokenRange(sourceTextForToken(analysisURI, analysisText, overlay, node.Declaration), node.Declaration)
	name := node.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	detail := node.Module
	kind := 12 // SymbolKind.Function
	if node.ImplTarget != "" {
		detail = "impl " + node.ImplTarget
		kind = 6 // SymbolKind.Method
	}
	return callHierarchyItem{
		Name:           name,
		Kind:           kind,
		Detail:         detail,
		URI:            uri,
		Range:          rng,
		SelectionRange: rng,
		Data:           callHierarchyData{AnalysisURI: analysisURI, NodeID: node.ID},
	}
}
