package sema

import "sec/internal/ast"

// inspectStateCorrelation consumes the owning state obligation and live checks
// at its exact program point. Another binding's matching state can explain a
// copy/paste mistake, but cannot establish invalidity or replace the normative
// state/discardability owner. Independently proven uses remain suppressed.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls",
// "Diagnostic ownership and coalescing", "Suppressing evidence";
// rules/errors/errorhandling.md — §§6.1–6.2.
func (b *pitfallBuilder) inspectStateCorrelation(operation ast.Expression) {
	fact, found := b.analyzer.ResolvedStateRequirementOf(operation)
	if !found {
		return
	}
	for _, check := range fact.Checks {
		if check.Binding.ID == fact.Binding.ID || check.Variant != fact.Variant || !sameConcreteType(check.Binding.Type, fact.Binding.Type) {
			continue
		}
		finding := PitfallFinding{
			Rule: PitfallWrongStateSubject, Family: PitfallOptionResultFlow,
			Classification: PitfallLikelyMistake, Confidence: PitfallConfidenceHigh,
			Subject: PitfallSubject{Expression: operation.String(), Source: expressionToken(operation)},
			EvidenceFor: []PitfallEvidence{
				{Strength: PitfallEvidenceProof, Fact: "the path checks " + check.Binding.Name + " for state " + check.Variant, Source: check.Source},
				{Strength: PitfallEvidenceProof, Fact: "this operation requires state " + fact.Variant + " of a different resolved binding, " + fact.Binding.Name, Source: expressionToken(operation)},
			},
			OwningRule: "state-projection",
			Actions:    []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "check the state of " + fact.Binding.Name + " before using its state-sensitive projection", Source: check.Source}},
		}
		if fact.Proven {
			evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "the owning state analysis independently proves the used binding's required state", Source: expressionToken(operation)}
			finding.State = PitfallStateSuppressed
			finding.EvidenceAgainst = []PitfallEvidence{evidence}
			finding.Suppression = &PitfallSuppression{Reason: "the used binding has its own canonical state proof", Evidence: []PitfallEvidence{evidence}}
		}
		b.add(finding)
		return // One deterministic checked subject explains this operation.
	}
}
