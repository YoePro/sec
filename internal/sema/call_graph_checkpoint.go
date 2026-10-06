package sema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"sec/internal/lexer"
)

const callGraphCheckpointVersion = 2

type graphCheckpoint struct {
	Format  string
	Version int
	Payload json.RawMessage
	SHA256  string
}
type graphCheckpointEntry struct {
	Scope        CallGraphScope
	Dependencies []CallGraphDependency
	Graph        persistedCallGraph
}
type persistedCallGraph struct {
	Scope    CallGraphScope
	Nodes    []CallableNode
	Bodies   map[CallableBodyID]CallableID
	Sites    []CallSite
	Roots    []CallRoot
	Arena    map[CallableID][]ArenaEffectSite
	Effects  map[CallableID][]EffectSite
	Blocking map[CallableID][]BlockEffectSite
}

// Checkpoint serializes only committed graphs, binding metadata and graph
// payload with a versioned integrity envelope. The caller owns atomic storage;
// partial or mixed bytes cannot be restored as a current positive graph.
// Rules: rules/compiler/incremental_compilation.md — §§31–34;
// rules/analysis/call_graph.md — "Graph data model", "Incremental compilation".
func (index *WorkspaceCallGraphIndex) Checkpoint() ([]byte, error) {
	if index == nil {
		return nil, errors.New("missing workspace graph index")
	}
	index.mu.RLock()
	defer index.mu.RUnlock()
	scopes := make([]CallGraphScope, 0, len(index.entries))
	for scope := range index.entries {
		scopes = append(scopes, scope)
	}
	sortGraphScopes(scopes)
	entries := make([]graphCheckpointEntry, 0, len(scopes))
	for _, scope := range scopes {
		entry := index.entries[scope]
		graph := entry.graph
		roots := make([]CallRoot, 0, len(graph.rootOrder))
		for _, id := range graph.rootOrder {
			roots = append(roots, graph.roots[id])
		}
		entries = append(entries, graphCheckpointEntry{scope, entry.dependencies, persistedCallGraph{graph.scope, graph.Nodes(), graph.bodyNodes, graph.sites, roots, graph.arenaEffects, graph.effects, graph.blockEffects}})
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(payload)
	return json.Marshal(graphCheckpoint{"sec.workspace-call-graph", callGraphCheckpointVersion, payload, hex.EncodeToString(digest[:])})
}

// RestoreWorkspaceCallGraphIndex validates an entire checkpoint before making
// any graph visible. Only exact supplied current scope/dependency fingerprints
// survive; missing, stale, differently planned or differently verified entries
// become misses. Decoding/integrity errors are cache errors, never Sec errors.
// Fingerprints must come from current producer observation, not the checkpoint.
// Rules: rules/compiler/incremental_compilation.md — §§4–5, 31–34, 85;
// rules/analysis/call_graph.md — "One graph per `CompilationPlan`".
func RestoreWorkspaceCallGraphIndex(data []byte, current map[CallGraphScope][]CallGraphDependency) (*WorkspaceCallGraphIndex, error) {
	var checkpoint graphCheckpoint
	if err := decodeGraphCheckpoint(data, &checkpoint); err != nil {
		return nil, err
	}
	if checkpoint.Format != "sec.workspace-call-graph" || checkpoint.Version != callGraphCheckpointVersion {
		return nil, errors.New("incompatible graph checkpoint schema")
	}
	digest := sha256.Sum256(checkpoint.Payload)
	if checkpoint.SHA256 != hex.EncodeToString(digest[:]) {
		return nil, errors.New("graph checkpoint integrity mismatch")
	}
	var entries []graphCheckpointEntry
	if err := decodeGraphCheckpoint(checkpoint.Payload, &entries); err != nil {
		return nil, err
	}
	index := &WorkspaceCallGraphIndex{}
	seen := map[CallGraphScope]bool{}
	for _, entry := range entries {
		deps, err := normalizeGraphDependencies(entry.Scope, entry.Dependencies)
		if err != nil || seen[entry.Scope] {
			return nil, errors.New("invalid graph checkpoint scope/dependencies")
		}
		seen[entry.Scope] = true
		graph, err := restoreCheckpointGraph(entry.Graph)
		if err != nil || !validWorkspaceGraph(graph, deps) || !validTestRootScope(graph, entry.Scope) || graph.scope != entry.Scope {
			return nil, errors.New("invalid graph checkpoint identities or sources")
		}
		fresh, err := normalizeGraphDependencies(entry.Scope, current[entry.Scope])
		if err != nil || !sameGraphDependencies(deps, fresh) {
			continue
		}
		// Reject a mixed logical workspace rather than letting Begin silently
		// invalidate one of two allegedly current restored graph units.
		for _, dep := range fresh {
			if old, exists := index.observed[callGraphDependencyKey{dep.Kind, dep.Identity}]; exists && old != dep.Fingerprint {
				return nil, errors.New("conflicting current workspace dependency fingerprints")
			}
		}
		lease, err := index.Begin(entry.Scope, fresh)
		if err != nil {
			return nil, err
		}
		if published, err := index.Publish(lease, graph); err != nil || !published {
			return nil, errors.New("graph checkpoint publication rejected")
		}
	}
	return index, nil
}

// decodeGraphCheckpoint rejects trailing, partial and unsupported fields.
// Rules: rules/compiler/incremental_compilation.md — §§31–33, 85.
func decodeGraphCheckpoint(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing graph checkpoint data")
	}
	return nil
}

// restoreCheckpointGraph reconstructs canonical records without recomputing or
// weakening roots, target coverage, effect evidence or ordered arena events.
// Rules: rules/analysis/call_graph.md — "Graph data model";
// rules/compiler/incremental_compilation.md — §§31–32.
func restoreCheckpointGraph(wire persistedCallGraph) (*CallGraph, error) {
	graph := newCallGraph()
	graph.scope = wire.Scope
	for _, node := range wire.Nodes {
		if _, exists := graph.nodes[node.ID]; exists {
			return nil, errors.New("duplicate graph node")
		}
		graph.nodes[node.ID] = node
		graph.nodeOrder = append(graph.nodeOrder, node.ID)
	}
	graph.bodyNodes = wire.Bodies
	graph.sites = wire.Sites
	for _, site := range wire.Sites {
		if graph.siteIDs[site.ID] {
			return nil, errors.New("duplicate graph site")
		}
		graph.siteIDs[site.ID] = true
	}
	for _, root := range wire.Roots {
		if _, exists := graph.roots[root.ID]; exists {
			return nil, errors.New("duplicate graph root")
		}
		graph.roots[root.ID] = root
		graph.rootOrder = append(graph.rootOrder, root.ID)
	}
	graph.arenaEffects, graph.effects, graph.blockEffects = wire.Arena, wire.Effects, wire.Blocking
	return graph, nil
}

// validWorkspaceGraph validates identities, edge coverage and dependency-bound
// source occurrences. Producers still own successful Sema and complete positive,
// negative, membership, contract and configuration dependency discovery.
// Rules: rules/analysis/call_graph.md — "Call-site record", "Closed target set", "Roots";
// rules/compiler/incremental_compilation.md — §§11–16, 31–33.
func validWorkspaceGraph(graph *CallGraph, deps []CallGraphDependency) bool {
	if graph == nil {
		return false
	}
	if len(graph.nodeOrder) != len(graph.nodes) || len(graph.rootOrder) != len(graph.roots) || len(graph.sites) != len(graph.siteIDs) {
		return false
	}
	seenNodes := map[CallableID]bool{}
	for _, id := range graph.nodeOrder {
		if seenNodes[id] || graph.nodes[id].ID != id {
			return false
		}
		seenNodes[id] = true
	}
	seenRoots := map[CallRootID]bool{}
	for _, id := range graph.rootOrder {
		if seenRoots[id] || graph.roots[id].ID != id {
			return false
		}
		seenRoots[id] = true
	}
	sources := map[string]bool{}
	for _, dep := range deps {
		if dep.Kind == "source" {
			sources[dep.Identity] = true
		}
	}
	source := func(token lexer.Token) bool { return sources[token.File] && token.Line > 0 && token.Column > 0 }
	for _, node := range graph.Nodes() {
		if node.ID == "" || !source(node.Declaration) {
			return false
		}
	}
	for body, target := range graph.bodyNodes {
		if body == "" {
			return false
		}
		if _, exists := graph.nodes[target]; !exists {
			return false
		}
	}
	for _, site := range graph.sites {
		if site.ID == "" || !source(site.Source) {
			return false
		}
		if _, exists := graph.nodes[site.Caller]; !exists {
			return false
		}
		switch site.Dispatch {
		case CallDispatchGenerated, CallDispatchDirect, CallDispatchStaticMethod, CallDispatchFunctionValue, CallDispatchClosure, CallDispatchForeign:
		default:
			return false
		}
		switch site.Execution {
		case CallExecutionSynchronous, CallExecutionDeferred, CallExecutionSpawnTask, CallExecutionSpawnThread, CallExecutionSpawnProcess:
		default:
			return false
		}
		targets := map[CallableID]bool{}
		for _, target := range site.Targets {
			if _, exists := graph.nodes[target]; !exists || targets[target] {
				return false
			}
			targets[target] = true
		}
		covered := map[CallableID]bool{}
		bodies := map[CallableBodyID]bool{}
		for _, body := range site.TargetSet.KnownTargets {
			if bodies[body] {
				return false
			}
			bodies[body] = true
			target, exists := graph.bodyNodes[body]
			if !exists || !targets[target] {
				return false
			}
			covered[target] = true
		}
		if len(covered) != len(targets) {
			return false
		}
		if site.TargetSet.IsClosed && (len(targets) == 0 || site.TargetSet.HasOpenContract || site.TargetSet.OpenContract != "") {
			return false
		}
		if !site.TargetSet.IsClosed && (!site.TargetSet.HasOpenContract || site.TargetSet.OpenContract == "") {
			return false
		}
	}
	for _, root := range graph.roots {
		if root.ID == "" || !source(root.Source) {
			return false
		}
		if _, exists := graph.nodes[root.Node]; !exists {
			return false
		}
	}
	for caller, effects := range graph.effects {
		if _, exists := graph.nodes[caller]; !exists {
			return false
		}
		for _, effect := range effects {
			if effect.Kind == "" || !source(effect.Source) {
				return false
			}
		}
	}
	for caller, effects := range graph.arenaEffects {
		if _, exists := graph.nodes[caller]; !exists {
			return false
		}
		for _, effect := range effects {
			if effect.Kind == "" || !source(effect.Source) {
				return false
			}
		}
	}
	for caller, effects := range graph.blockEffects {
		if _, exists := graph.nodes[caller]; !exists {
			return false
		}
		for _, effect := range effects {
			if effect.Kind == "" || !source(effect.Source) {
				return false
			}
		}
	}
	return true
}
