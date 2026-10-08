package sema

import (
	"fmt"

	"sec/internal/ast"
)

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

// inspectForeignExtentOrigin reports a relational intent mistake only when the
// canonical contract pairs these arguments and their known Places are disjoint.
// Overlap, equal origins and uncertainty cannot establish a mismatch. This is
// advisory: unrelated extents can coincide numerically, so it is not an ABI or
// bounds-invalidity proof and no automatic replacement is authorized.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pointer/extent provenance mismatch",
// "FFI pointer/size origin mismatch", "Classification", "Fix safety".
func (b *pitfallBuilder) inspectForeignExtentOrigin(fact ResolvedForeignBufferExtent) {
	if fact.PointerOrigin == nil || fact.ExtentOrigin == nil || Relationship(*fact.PointerOrigin, *fact.ExtentOrigin) != PlaceDisjoint {
		return
	}
	b.add(PitfallFinding{
		Rule: PitfallForeignExtentOrigin, Family: PitfallForeignContracts,
		Classification: PitfallLikelyMistake, Confidence: PitfallConfidenceHigh,
		Subject: PitfallSubject{Expression: fact.PointerExpression + ", " + fact.ExtentExpression, Source: fact.Call},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: fmt.Sprintf("the foreign contract pairs pointer argument %d with extent argument %d in %s", fact.PointerArgument, fact.ExtentArgument, fact.ExtentUnit), Source: fact.ContractSource},
			{Strength: PitfallEvidenceProof, Fact: "the pointer derives from " + fact.PointerOrigin.String(), Source: fact.PointerSource},
			{Strength: PitfallEvidenceProof, Fact: "the extent derives from disjoint storage " + fact.ExtentOrigin.String(), Source: fact.ExtentSource},
		},
		OwningRule: "foreign-buffer-extent-contract",
		Actions:    []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "check that the extent describes the storage addressed by the paired pointer; use the contract's extent units", Source: fact.ExtentSource}},
	})
}

// inspectForeignExtentUnit correlates independently resolved quantity units,
// exact element stride and the paired contract. A byte-wide element or known
// zero extent has equal numeric counts and cannot establish this pitfall.
// Partial transfers can be intentional, so unit guidance is advisory and is
// never an automatic arithmetic rewrite or a mandatory ABI/bounds error.
// Rules: rules/analysis/pitfall_analysis.md — "FFI element count versus byte count",
// "Classification", "Fix safety", "Diagnostic ownership and coalescing".
func (b *pitfallBuilder) inspectForeignExtentUnit(fact ResolvedForeignBufferExtent) {
	if fact.SuppliedExtentUnit == "" || fact.ElementStorageBytes <= 1 || fact.ZeroExtent || fact.SuppliedExtentUnit == fact.ExtentUnit {
		return
	}
	b.add(PitfallFinding{
		Rule: PitfallForeignExtentUnit, Family: PitfallForeignContracts,
		Classification: PitfallLikelyMistake, Confidence: PitfallConfidenceHigh,
		Subject: PitfallSubject{Expression: fact.ExtentExpression, Source: fact.Call},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: fmt.Sprintf("the paired foreign extent argument %d is measured in %s", fact.ExtentArgument, fact.ExtentUnit), Source: fact.ContractSource},
			{Strength: PitfallEvidenceProof, Fact: "the supplied quantity describes the same pointer receiver in " + string(fact.SuppliedExtentUnit), Source: fact.ExtentSource},
			{Strength: PitfallEvidenceProof, Fact: fmt.Sprintf("the resolved element storage stride is %d bytes, so nonzero element and byte counts differ", fact.ElementStorageBytes), Source: fact.PointerSource},
		},
		OwningRule: "foreign-buffer-extent-contract",
		Actions:    []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "check whether a partial transfer is intentional; otherwise supply the extent in " + string(fact.ExtentUnit) + " with checked size arithmetic", Source: fact.ExtentSource}},
	})
}
