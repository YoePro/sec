package sema

import "fmt"

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
