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
	if !ok || identifier == nil || maybeEmptySubject != "" {
		return nil
	}
	for _, variant := range provableMatchVariants(subjectType) {
		if active, known := a.knownVariantTest(identifier.Value, variant); known && active {
			return a.variantProofProvenance(identifier.Value, variant)
		}
	}
	return nil
}

// provableMatchVariants lists the top-level states a reachable-state proof
// can select: the variants of a union, Some/None of an Option and Ok/Err of
// a Result (flowcontrol_match.md § 5 names every legal Option and Result
// case alongside union variants).
func provableMatchVariants(subjectType Type) []string {
	switch {
	case isOptionType(subjectType):
		return []string{"Some", "None"}
	case subjectType.Kind == UnionType:
		names := make([]string, 0, len(subjectType.UnionVariants))
		for _, variant := range subjectType.UnionVariants {
			names = append(names, variant.Name)
		}
		return names
	case subjectType.Kind == ResultType:
		return []string{"Ok", "Err"}
	}
	return nil
}

// matchArmTopState names the top-level Option/Result state an arm pattern
// selects ("Some", "None", "Ok" or "Err"), or "" for any other arm.
func matchArmTopState(subjectType Type, info matchPatternInfo) string {
	if !isOptionType(subjectType) && subjectType.Kind != ResultType {
		return ""
	}
	if isErrorVariantMatchKey(info.Variant) {
		return "Err"
	}
	switch info.Kind {
	case "Some", "None", "Ok", "Err":
		return info.Kind
	}
	return ""
}

// variantProofProvenance records how knownVariantTest proved variant active
// for binding: its construction, or the dominating `is` test it found.
func (a *Analyzer) variantProofProvenance(binding string, variant string) *ResolvedVariantProof {
	symbol := a.symbols.get(binding)
	proof := &ResolvedVariantProof{Binding: binding, Variant: variant, Basis: VariantProofConstruction, Source: symbol.Token}
	if symbol.ConstructedVariant != "" && !symbol.Mutable {
		return proof
	}
	for _, active := range a.activeConditionFacts {
		if !a.variantFactUsable(active, symbol) {
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
