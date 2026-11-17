package main

import (
	"sort"

	"sec/internal/sema"
)

// addTrustProvenance presents compiler-owned trust facts for the selected
// program or reachable function scope. This is optional verbose evidence, not a
// diagnostic, safety certificate or new callable-type/effect contract. Open
// targets remain explicitly unknown; ordinary safe wrappers retain their actual
// implementation trust dependencies.
// Rules: rules/memory/unsafe.md — §§3.4,14,52(1),54(7);
// rules/tooling/cli.md — §§5.3,5.4(2–4).
func (r *analyseReport) addTrustProvenance(analyzer *sema.Analyzer, graph *sema.CallGraph, nodes []sema.CallableNode, names map[sema.CallableID]string) {
	section := r.section("trust provenance")
	section.add("Recorded implementation dependencies; absence of a recorded dependency is not a trust-freedom proof.")
	ordered := append([]sema.CallableNode(nil), nodes...)
	sort.Slice(ordered, func(i, j int) bool {
		if names[ordered[i].ID] != names[ordered[j].ID] {
			return names[ordered[i].ID] < names[ordered[j].ID]
		}
		return ordered[i].ID < ordered[j].ID
	})
	for _, node := range ordered {
		name := names[node.ID]
		facts := analyzer.TrustProvenance(node.ID)
		if node.Extern {
			section.add("%s: trusted foreign declaration at %s; implementation outside Sec source verification", name, analysePosition(node.Declaration))
		} else {
			safety := "safe"
			if facts.CallerFacingUnsafe {
				safety = "unsafe"
			}
			section.add("%s: caller-facing safety %s at %s", name, safety, analysePosition(node.Declaration))
		}
		// unsafe.md §15: authorization never hides known effects or uncertainty.
		for _, effect := range facts.Effects.Effects {
			section.add("%s: effects remain active through unsafe: %s", name, effect)
		}
		for _, effect := range facts.Effects.UnknownEffects {
			section.add("%s: effect unknown through unsafe: %s; no effect-freedom proof", name, effect)
		}
		for _, site := range facts.Direct {
			section.add("%s: unsafe operation %s (%s) at %s; authorized=%t", name, site.Operation, site.Kind, analysePosition(site.Token), site.Authorized)
			for _, obligation := range site.Obligations {
				section.add("%s: operation obligation for %s: %s", name, site.Operation, obligation)
			}
		}
		for _, foreign := range facts.Foreign {
			declaration, _ := graph.Node(foreign)
			section.add("%s: calls trusted foreign %s at %s", name, names[foreign], analysePosition(declaration.Declaration))
		}
		if len(facts.Path) > 0 {
			section.add("%s: implementation depends on trust via %s", name, analysePath(facts.Path, names))
		}
		if len(facts.UnknownPath) > 1 {
			section.add("%s: trust dependencies unknown via %s; open callable contract retained", name, analysePath(facts.UnknownPath, names))
		}
		for _, site := range graph.Outgoing(node.ID) {
			if site.TargetSet.HasOpenContract {
				section.add("%s: trust dependencies unknown beyond open callable target at %s; declared contract retained", name, analysePosition(site.Source))
			}
		}
	}
}
