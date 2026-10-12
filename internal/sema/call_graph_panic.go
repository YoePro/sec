package sema

// EffectSummary returns direct semantic effects and a deterministic shortest
// synchronous cause path for transitive panic behavior, including reachable
// checked unreachable statements.
//
// Rules:
//   - rules/errors/panic.md — § 16(5)–(6) "Checked unreachable"
//   - rules/errors/panic.md — §§21,25(3)–(4),28(2) "@noPanic", causes and provenance
//   - rules/analysis/effect_analysis.md — "Effect causes"
func (g *CallGraph) EffectSummary(id CallableID) CallableEffectSummary {
	if g == nil {
		return CallableEffectSummary{}
	}
	summary := CallableEffectSummary{DirectEffects: cloneEffectSites(g.effects[id])}
	summary.PanicPath = g.synchronousPathTo(id, func(candidate CallableID) bool {
		for _, effect := range g.effects[candidate] {
			if isPanicEffectKind(effect.Kind) {
				return true
			}
		}
		return false
	})
	// Consume the canonical domain propagation, preserving every registered
	// panic cause rather than independently traversing or guessing call edges.
	for _, cause := range g.DomainSummary(id).Causes {
		if cause.Effect != nil && isPanicEffectKind(cause.Effect.Kind) {
			summary.PanicCauses = append(summary.PanicCauses, cause)
		}
	}
	summary.MayPanic = len(summary.PanicPath) > 0
	return summary
}
