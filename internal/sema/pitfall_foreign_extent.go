package sema

import "sec/internal/ast"

// ForeignExtentInputs exposes the contract-backed argument relationships
// consumed in budget-admitted pitfall bodies. An input is evidence for future
// FFI rules, not a diagnostic or a claim that units are correct; represented
// member origins support the separate pointer/extent provenance rule.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships",
// "FFI pointer/extent provenance mismatch", "FFI element count versus byte count".
func (p *PitfallAnalysis) ForeignExtentInputs() []ResolvedForeignBufferExtent {
	if p == nil {
		return nil
	}
	return cloneForeignBufferExtents(p.foreignExtentInputs)
}

// consumeForeignBufferExtents uses only the owning producer's completed facts.
// Missing metadata stays unknown; pointer/length-looking names confer no fact.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis",
// "Canonical foreign extent relationships", "Interactive, Standard, and Deep analysis".
func (b *pitfallBuilder) consumeForeignBufferExtents(call *ast.CallExpression) {
	if facts, known := b.analyzer.ResolvedForeignBufferExtentsOf(call); known {
		b.result.foreignExtentInputs = append(b.result.foreignExtentInputs, facts...)
		for _, fact := range facts {
			b.inspectForeignExtentOrigin(fact)
			b.inspectForeignExtentUnit(fact)
		}
	}
}
