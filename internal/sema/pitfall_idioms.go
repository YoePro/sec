package sema

import "sort"

// PitfallCanonicalIdiom identifies a preferred language construct in a
// suggestion. It is presentation metadata, independent of confidence, source
// validity and the evidence required to authorize an automatic fix.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance",
// "Fix safety", "Diagnostic ownership and coalescing".
type PitfallCanonicalIdiom string

const (
	PitfallIdiomRangeMembership   PitfallCanonicalIdiom = "range-membership"
	PitfallIdiomHalfOpenTraversal PitfallCanonicalIdiom = "half-open-traversal"
)

// preferPitfallCanonicalIdioms marks only canonical replacements from the
// semantic producers that own these rewrites, then stably orders them before
// other suggestions. Suppressed findings and explanation-only actions have no
// idiom recommendation. This does not upgrade SuggestedEdit to ProvenFix or
// change finding severity, evaluation depth or ordinary expression validity.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance",
// "Strong advisory scope", "Fix safety", "Required control-flow tests".
func preferPitfallCanonicalIdioms(finding *PitfallFinding) {
	for i := range finding.Actions {
		action := &finding.Actions[i]
		action.Idiom = ""
		if finding.State == PitfallStateSuppressed || action.Replacement == "" {
			continue
		}
		switch finding.Rule {
		case PitfallRangeMembershipIdiom:
			action.Idiom = PitfallIdiomRangeMembership
		case PitfallInclusiveLengthIndex, PitfallFragileInclusiveLength, PitfallOmittedLastElement:
			action.Idiom = PitfallIdiomHalfOpenTraversal
		}
	}
	sort.SliceStable(finding.Actions, func(i, j int) bool {
		return finding.Actions[i].Idiom != "" && finding.Actions[j].Idiom == ""
	})
}
