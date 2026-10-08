package sema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// CallGraphScope separates graph validity from within-snapshot callable IDs.
// CompilerModel must identify compatible semantic rules, toolchain and verifier.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`";
// rules/compiler/incremental_compilation.md — §§5, 31–32.
type CallGraphScope struct{ Module, CompilationPlan, CompilerModel string }

// CallGraphDependency includes positive, set-valued and negative dependency
// fingerprints. Producers must supply the complete closure, including source
// membership, imported contracts and target decisions; graph edges alone do
// not establish this closure. Identities are canonical producer keys.
// Rules: rules/compiler/incremental_compilation.md — §§11–16;
// rules/analysis/call_graph.md — "Incremental compilation".
type CallGraphDependency struct{ Kind, Identity, Fingerprint string }

type callGraphDependencyKey struct{ kind, identity string }
type workspaceGraphEntry struct {
	dependencies []CallGraphDependency
	graph        *CallGraph
}

// CallGraphLease is an opaque, single-publication analysis generation.
// Rules: rules/compiler/incremental_compilation.md — §29.
type CallGraphLease struct {
	owner        *WorkspaceCallGraphIndex
	scope        CallGraphScope
	generation   uint64
	dependencies []CallGraphDependency
}

// WorkspaceCallGraphIndex owns detached current graphs across analysis runs.
// Changed dependencies evict whole affected graph units, including all their
// SCC/summary/reachability consumers; unrelated modules/plans remain reusable.
// It is a compiler service, not a second tooling-owned semantic graph.
// Rules: rules/analysis/call_graph.md — "Incremental compilation", "Graph update phases";
// rules/compiler/incremental_compilation.md — §§4–6, 29–34.
type WorkspaceCallGraphIndex struct {
	mu         sync.RWMutex
	generation map[CallGraphScope]uint64
	pending    map[CallGraphScope]CallGraphLease
	entries    map[CallGraphScope]workspaceGraphEntry
	observed   map[callGraphDependencyKey]string
}

// AnalyzeIndexed runs ordinary Sema and commits its canonical graph only after
// successful analysis and latest-generation verification. Cache infrastructure
// failures remain separate from Sec errors; neither failed nor superseded work
// publishes a current graph. No other Analyzer facts are reused from this cache.
// Rules: rules/analysis/call_graph.md — "Graph update phases", "Unreachable code";
// rules/compiler/incremental_compilation.md — §§4, 29–33, 39, 72.
func (analyzer *Analyzer) AnalyzeIndexed(program *ast.Program, index *WorkspaceCallGraphIndex, scope CallGraphScope, dependencies []CallGraphDependency) ([]Error, bool, error) {
	lease, cacheError := index.Begin(scope, dependencies)
	errors := analyzer.Analyze(program)
	if cacheError != nil {
		return errors, false, cacheError
	}
	if len(errors) != 0 {
		index.Discard(lease)
		return errors, false, nil
	}
	published, cacheError := index.Publish(lease, analyzer.CallGraph())
	if cacheError != nil {
		index.Discard(lease)
	}
	return errors, published, cacheError
}

// Discard cancels only its own current lease, preserving newer pending work.
// Rules: rules/compiler/incremental_compilation.md — §§29, 72, 86.
func (index *WorkspaceCallGraphIndex) Discard(lease CallGraphLease) {
	if index == nil || lease.owner != index {
		return
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	current, exists := index.pending[lease.scope]
	if exists && current.generation == lease.generation {
		delete(index.pending, lease.scope)
	}
}

// Begin records complete current dependency facts and supersedes older work.
// Changes also invalidate dependent modules/plans, including in-flight leases.
// Rules: rules/analysis/call_graph.md — "Incremental compilation";
// rules/compiler/incremental_compilation.md — §§14, 29–30.
func (index *WorkspaceCallGraphIndex) Begin(scope CallGraphScope, dependencies []CallGraphDependency) (CallGraphLease, error) {
	deps, err := normalizeGraphDependencies(scope, dependencies)
	if index == nil || err != nil {
		return CallGraphLease{}, errors.New("graph indexing requires an index, module, plan, compiler model and complete fingerprinted dependencies")
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	if index.entries == nil {
		index.entries = map[CallGraphScope]workspaceGraphEntry{}
		index.pending = map[CallGraphScope]CallGraphLease{}
		index.generation = map[CallGraphScope]uint64{}
		index.observed = map[callGraphDependencyKey]string{}
	}
	for _, dep := range deps {
		key := callGraphDependencyKey{dep.Kind, dep.Identity}
		if previous, exists := index.observed[key]; exists && previous != dep.Fingerprint {
			index.invalidateLocked(key)
		}
		index.observed[key] = dep.Fingerprint
	}
	index.generation[scope]++
	lease := CallGraphLease{owner: index, scope: scope, generation: index.generation[scope], dependencies: deps}
	index.pending[scope] = lease
	delete(index.entries, scope)
	return lease, nil
}

// Publish accepts only the latest live lease and an internally consistent graph.
// The producer must have completed successful semantic analysis and verified
// dependency closure. Invalid/error-recovery graphs are not cacheable proofs.
// Rules: rules/compiler/incremental_compilation.md — §§29–33;
// rules/analysis/call_graph.md — "Graph update phases", "Unreachable code".
func (index *WorkspaceCallGraphIndex) Publish(lease CallGraphLease, graph *CallGraph) (bool, error) {
	if index == nil || lease.owner != index {
		return false, errors.New("graph lease belongs to another index")
	}
	detached, scopeError := graph.ForCompilationPlan(lease.scope)
	if scopeError != nil {
		return false, scopeError
	}
	if graph == nil || !validWorkspaceGraph(detached, lease.dependencies) || !validTestRootScope(detached, lease.scope) {
		return false, errors.New("graph does not match its complete source and identity dependencies")
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	current, exists := index.pending[lease.scope]
	if !exists || current.generation != lease.generation {
		return false, nil
	}
	index.entries[lease.scope] = workspaceGraphEntry{append([]CallGraphDependency(nil), lease.dependencies...), detached}
	delete(index.pending, lease.scope)
	return true, nil
}

// Snapshot requires exact current dependency compatibility before reuse. Misses
// are infrastructure states, not source errors. Returned facts are detached.
// Rules: rules/compiler/incremental_compilation.md — §§4–5, 31–32;
// rules/analysis/call_graph.md — "One graph per `CompilationPlan`".
func (index *WorkspaceCallGraphIndex) Snapshot(scope CallGraphScope, dependencies []CallGraphDependency) (*CallGraph, bool) {
	deps, err := normalizeGraphDependencies(scope, dependencies)
	if index == nil || err != nil {
		return nil, false
	}
	index.mu.RLock()
	defer index.mu.RUnlock()
	entry, exists := index.entries[scope]
	if !exists || !sameGraphDependencies(entry.dependencies, deps) {
		return nil, false
	}
	return entry.graph.clone(), true
}

// Invalidate rejects cached and in-flight results depending on an edited
// source, contract, source-set/negative probe, or configuration identity.
// Rules: rules/analysis/call_graph.md — "Incremental compilation";
// rules/compiler/incremental_compilation.md — §§11–14, 29.
func (index *WorkspaceCallGraphIndex) Invalidate(kind, identity string) []CallGraphScope {
	if index == nil {
		return nil
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	key := callGraphDependencyKey{kind, identity}
	delete(index.observed, key)
	return index.invalidateLocked(key)
}

// invalidateLocked evicts whole dependent canonical graph units so no retained
// derived view can mix a changed callee/contract with its old transitive callers.
// Rules: rules/analysis/call_graph.md — "Incremental compilation".
func (index *WorkspaceCallGraphIndex) invalidateLocked(key callGraphDependencyKey) []CallGraphScope {
	affected := map[CallGraphScope]bool{}
	has := func(deps []CallGraphDependency) bool {
		for _, dep := range deps {
			if dep.Kind == key.kind && dep.Identity == key.identity {
				return true
			}
		}
		return false
	}
	for scope, entry := range index.entries {
		if has(entry.dependencies) {
			affected[scope] = true
		}
	}
	for scope, lease := range index.pending {
		if has(lease.dependencies) {
			affected[scope] = true
		}
	}
	scopes := make([]CallGraphScope, 0, len(affected))
	for scope := range affected {
		delete(index.entries, scope)
		delete(index.pending, scope)
		index.generation[scope]++
		scopes = append(scopes, scope)
	}
	sortGraphScopes(scopes)
	return scopes
}

// normalizeGraphDependencies rejects incomplete or ambiguous validity metadata
// and detaches the ordered canonical fingerprint set. "source" identities match
// lexer source paths; other kinds represent producer-owned semantic dependencies.
// Rules: rules/compiler/incremental_compilation.md — §§11–16, 31–32.
func normalizeGraphDependencies(scope CallGraphScope, dependencies []CallGraphDependency) ([]CallGraphDependency, error) {
	if scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" || len(dependencies) == 0 {
		return nil, errors.New("missing graph scope or dependencies")
	}
	deps := append([]CallGraphDependency(nil), dependencies...)
	seen := map[callGraphDependencyKey]bool{}
	for _, dep := range deps {
		key := callGraphDependencyKey{dep.Kind, dep.Identity}
		if dep.Kind == "" || dep.Identity == "" || dep.Fingerprint == "" || seen[key] {
			return nil, errors.New("missing or duplicate dependency fingerprint")
		}
		seen[key] = true
	}
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Kind != deps[j].Kind {
			return deps[i].Kind < deps[j].Kind
		}
		return deps[i].Identity < deps[j].Identity
	})
	return deps, nil
}

// sameGraphDependencies compares full fingerprint sets, including membership
// additions/removals; equal canonical node IDs alone never authorize reuse.
// Rules: rules/compiler/incremental_compilation.md — §§5, 12–16.
func sameGraphDependencies(a, b []CallGraphDependency) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sortGraphScopes makes invalidation and checkpoint order independent of maps.
// Rules: rules/compiler/incremental_compilation.md — §15.
func sortGraphScopes(scopes []CallGraphScope) {
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Module != scopes[j].Module {
			return scopes[i].Module < scopes[j].Module
		}
		if scopes[i].CompilationPlan != scopes[j].CompilationPlan {
			return scopes[i].CompilationPlan < scopes[j].CompilationPlan
		}
		return scopes[i].CompilerModel < scopes[j].CompilerModel
	})
}

const callGraphCheckpointVersion = 3

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
		case CallDispatchInterface, CallDispatchGenerated, CallDispatchDirect, CallDispatchStaticMethod, CallDispatchFunctionValue, CallDispatchClosure, CallDispatchForeign:
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
		if contract := site.TargetSet.Contract; contract != nil {
			if !site.TargetSet.HasOpenContract || site.TargetSet.IsClosed || contract.ID != site.TargetSet.OpenContract || !validGraphCallableContract(contract, graph.scope) {
				return false
			}
			for _, parameter := range contract.Parameters {
				if parameter.TypeIdentity == "" {
					return false
				}
			}
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
