package sema

import (
	"errors"
	"sort"
	"sync"

	"sec/internal/ast"
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
