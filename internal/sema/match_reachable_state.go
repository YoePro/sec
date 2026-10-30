package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// provenMatchVariant returns the active variant of a union match subject when
// it is soundly proven at the match point: the subject is an immutable binding
// whose variant is known from its construction or from a dominating,
// unmutated `is` fact (knownVariantTest). A mutable, possibly-empty or
// non-binding subject, or any other union shape, proves nothing, so coverage
// then uses the whole declared domain.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 5 "Reachable-state exhaustiveness (MD-042)"
//   - rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§ 8(1), 8(6)–8(7)
func (a *Analyzer) provenMatchVariant(subject ast.Expression, subjectType Type, maybeEmptySubject string) *ResolvedVariantProof {
	identifier, ok := subject.(*ast.Identifier)
	if !ok || identifier == nil || subjectType.Kind != UnionType || maybeEmptySubject != "" {
		return nil
	}
	for _, variant := range subjectType.UnionVariants {
		if active, known := a.knownVariantTest(identifier.Value, variant.Name); known && active {
			return a.variantProofProvenance(identifier.Value, variant.Name)
		}
	}
	return nil
}

// variantProofProvenance records how knownVariantTest proved variant active
// for binding: its construction, or the dominating `is` test it found.
func (a *Analyzer) variantProofProvenance(binding string, variant string) *ResolvedVariantProof {
	symbol := a.symbols[binding]
	proof := &ResolvedVariantProof{Binding: binding, Variant: variant, Basis: VariantProofConstruction, Source: symbol.Token}
	if symbol.ConstructedVariant != "" {
		return proof
	}
	for _, active := range a.activeConditionFacts {
		if active.epoch != a.arrayIndexMutationEpoch || active.fact.Condition == nil || active.fact.Kind == ConditionFactLogicalRHSFalse {
			continue
		}
		for _, test := range conjunctStateTests(active.fact.Condition) {
			subject, ok := test.Subject.(*ast.Identifier)
			if ok && subject != nil && subject.Value == binding && !test.Empty && !test.Negated && test.Variant != nil && test.Variant.Value == variant {
				proof.Basis, proof.Source = VariantProofStateTest, test.Token
				return proof
			}
		}
	}
	return proof
}

// reportInactiveMatchArm explains a legal arm whose variant the proven subject
// state excludes. The arm keeps full type, guard and ownership checking; only
// its selection is impossible here.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 25A "Proven-inactive arms (MD-042)"
//   - rules/tooling/diagnostics.md — § 24(4e)
func (a *Analyzer) reportInactiveMatchArm(arm *ast.MatchArm, subject ast.Expression, subjectType Type, variant string, active string) {
	a.addWarningAtTokenWithMetadata(arm.Pattern.Token, diagnostics.InactiveMatchArm,
		fmt.Sprintf("%s is proven to contain %s.%s at this match point. %s.%s remains a valid variant and this arm is legal; keep it for clarity or remove it.", subject.String(), typeDisplayName(subjectType), active, typeDisplayName(subjectType), variant),
		"this match arm cannot be selected for the current state of %s", subject.String())
}
