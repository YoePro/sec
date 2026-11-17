package sema

import (
	"sort"

	"sec/internal/lexer"
)

// EffectCause is one non-panic effect kind reachable from a callable on a
// synchronous path: the shortest deterministic call path to the callable
// that introduces it and the introducing source site. Panic effects keep
// their own registered-reason causes (CallableEffectSummary.PanicCauses).
type EffectCause struct {
	Kind   EffectKind
	Path   []CallableID
	Source lexer.Token
}

// EffectCauses returns, for every non-panic effect kind that a synchronous
// call from id can reach (ordinary and volatile memory access, atomic
// fences, nondeterministic input and any later registered kind), its
// canonical cause chain from the one compiler-owned call graph, so the LSP,
// sec analyse and diagnostics present the same chain. The result is ordered
// by effect kind.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 109(1)–(3) "effect cause", one canonical effect model
//   - rules/analysis/call_graph.md — "Same-stack effect transfer", "Diagnostics"
//   - rules/tooling/lsp.md — § 10.2(1), § 24.4(2) navigable cause chains
func (g *CallGraph) EffectCauses(id CallableID) []EffectCause {
	if g == nil || id == "" {
		return nil
	}
	kinds := map[EffectKind]bool{}
	for _, sites := range g.effects {
		for _, site := range sites {
			if !isPanicEffectKind(site.Kind) {
				kinds[site.Kind] = true
			}
		}
	}
	ordered := make([]EffectKind, 0, len(kinds))
	for kind := range kinds {
		ordered = append(ordered, kind)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var causes []EffectCause
	for _, kind := range ordered {
		introduces := func(candidate CallableID) bool {
			_, ok := g.firstEffectSite(candidate, kind)
			return ok
		}
		path := g.synchronousPathTo(id, introduces)
		if len(path) == 0 {
			continue
		}
		site, _ := g.firstEffectSite(path[len(path)-1], kind)
		causes = append(causes, EffectCause{Kind: kind, Path: path, Source: site.Source})
	}
	return causes
}

// firstEffectSite returns the first recorded direct effect of kind in a
// callable, in source order.
func (g *CallGraph) firstEffectSite(id CallableID, kind EffectKind) (EffectSite, bool) {
	var first EffectSite
	found := false
	for _, site := range g.effects[id] {
		if site.Kind != kind {
			continue
		}
		if !found || site.Source.Line < first.Source.Line || site.Source.Line == first.Source.Line && site.Source.Column < first.Source.Column {
			first, found = site, true
		}
	}
	return first, found
}
