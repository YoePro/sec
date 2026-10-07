package sema

import "fmt"

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
