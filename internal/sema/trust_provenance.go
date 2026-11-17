package sema

import (
	"slices"
	"sort"
)

// TrustProvenance is the implementation trust provenance of one callable:
// its caller-facing safety, the unsafe operations in its own body, the
// trusted foreign declarations it calls directly, and the shortest call path
// to another callable that introduces trust. A safe wrapper keeps the
// provenance of what it wraps; it is analysis metadata, not part of the
// source-level callable type. Effects independently retains the canonical
// same-stack may-effect summary, not a safety or effect-freedom certificate.
type TrustProvenance struct {
	// CallerFacingUnsafe reports an unsafe fn/method, whose callers must
	// satisfy its obligations; otherwise the callable is safe to call.
	CallerFacingUnsafe bool
	// Effects is the canonical same-stack effect summary, independent of trust
	// and caller-facing safety. UnknownEffects never certifies effect freedom;
	// absent producer facts also do not constitute a verified guarantee.
	// Rules: rules/memory/unsafe.md §§13(3–6),15(1–5).
	Effects CallableDomainSummary
	// ForeignDeclaration identifies a trusted implementation outside Sec
	// source verification, including when this query is on the extern itself.
	ForeignDeclaration bool
	// Direct lists the unsafe operation sites in the callable's own body.
	Direct []UnsafeOperationSite
	// Foreign lists the extern declarations the callable calls directly.
	Foreign []CallableID
	// Path is the shortest deterministic call path from the callable to a
	// different callable that has direct unsafe operations or is a foreign
	// declaration; it is empty when no callee introduces trust.
	Path []CallableID
	// UnknownPath is a shortest deterministic witness to a callable with an
	// open target set. Its implementation trust is unknown, independently of
	// caller-facing safety or any separately recorded known trust dependency.
	// A direct open call has a one-element path containing this callable.
	UnknownPath []CallableID
}

// DependsOnTrust reports whether the implementation depends on any unsafe
// operation or trusted foreign declaration. False is not proof of trust
// freedom: UnknownPath separately records unresolved implementations.
// Rules: rules/memory/unsafe.md — §§3.4,14;
// rules/analysis/call_graph.md — §§44–45.
func (p TrustProvenance) DependsOnTrust() bool {
	return p.ForeignDeclaration || len(p.Direct) > 0 || len(p.Foreign) > 0 || len(p.Path) > 0
}

// TrustProvenance derives the trust provenance of a callable from the shared
// unsafe operation registry sites and the canonical call graph, over every
// represented execution relation (a spawned or deferred callee is part of
// the implementation). A foreign declaration is trusted: its implementation
// lies outside Sec source verification. Effects use the canonical same-stack
// domain summary; spawned bodies remain in their own execution context, while
// trust provenance still records dependencies through all execution relations.
//
// Rules:
//   - rules/memory/unsafe.md — § 3.4 "Trusted declaration"; §§13–15 "Trust provenance" and effects; § 52(1) LSP and tooling
//   - rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"
//   - rules/analysis/call_graph.md — §§44–45 (open contracts retain unknown implementations)
func (a *Analyzer) TrustProvenance(id CallableID) TrustProvenance {
	var provenance TrustProvenance
	if a == nil || a.callGraph == nil || id == "" {
		return provenance
	}
	graph := a.callGraph
	node, ok := graph.Node(id)
	if !ok {
		return provenance
	}
	provenance.Effects = graph.DomainSummary(id)
	provenance.CallerFacingUnsafe = node.Unsafe
	provenance.ForeignDeclaration = node.Extern
	direct := map[CallableID]bool{}
	for _, site := range a.unsafeSites {
		if site.Callable == "" {
			continue
		}
		direct[site.Callable] = true
		if site.Callable == id {
			site.Obligations = slices.Clone(site.Obligations)
			provenance.Direct = append(provenance.Direct, site)
		}
	}
	introduces := func(candidate CallableID) bool {
		if direct[candidate] {
			return true
		}
		target, ok := graph.Node(candidate)
		return ok && target.Extern
	}
	seen := map[CallableID]bool{}
	for _, target := range graph.completeExecutionTargets(id) {
		if callee, ok := graph.Node(target); ok && callee.Extern && !seen[target] {
			seen[target] = true
			provenance.Foreign = append(provenance.Foreign, target)
		}
	}
	sort.Slice(provenance.Foreign, func(i, j int) bool { return provenance.Foreign[i] < provenance.Foreign[j] })
	provenance.Path = trustPath(graph, id, introduces)
	hasOpenTarget := func(candidate CallableID) bool {
		for _, site := range graph.Outgoing(candidate) {
			if site.TargetSet.HasOpenContract {
				return true
			}
		}
		return false
	}
	if hasOpenTarget(id) {
		provenance.UnknownPath = []CallableID{id}
	} else {
		provenance.UnknownPath = trustPath(graph, id, hasOpenTarget)
	}
	return provenance
}

// trustPath is a breadth-first shortest witness from start to a different
// callable that introduces trust; recursion terminates through the visited
// set and targets are visited in identity order for determinism.
// Rules: rules/memory/unsafe.md — §14; rules/analysis/call_graph.md —
// "One canonical graph, multiple analysis views", §§44–45.
func trustPath(graph *CallGraph, start CallableID, introduces func(CallableID) bool) []CallableID {
	type entry struct {
		id   CallableID
		path []CallableID
	}
	queue := []entry{{id: start, path: []CallableID{start}}}
	visited := map[CallableID]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		targets := graph.completeExecutionTargets(current.id)
		sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
		for _, target := range targets {
			if visited[target] {
				continue
			}
			visited[target] = true
			path := append(append([]CallableID(nil), current.path...), target)
			if introduces(target) {
				return path
			}
			queue = append(queue, entry{id: target, path: path})
		}
	}
	return nil
}
