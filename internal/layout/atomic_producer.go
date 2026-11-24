package layout

import (
	"fmt"
	"strings"
)

// AtomicActivation is profile permission, independent of physical availability.
// Its zero value is missing evidence, never implicit permission.
// Rules: rules/concurrency/atomics.md — § 37A(11);
// rules/platform/platform_model.md — § 14.
type AtomicActivation string

const (
	AtomicActivationUnknown AtomicActivation = ""
	AtomicEnabled           AtomicActivation = "enabled"
	AtomicDisabled          AtomicActivation = "disabled"
	AtomicNotApplicable     AtomicActivation = "not-applicable"
)

// AtomicProfilePolicy applies to every atomic operation/context in this plan.
// A context-specific implementation cannot enable a globally disabled feature.
// Rules: rules/concurrency/atomics.md — § 37A(8), (11).
type AtomicProfilePolicy struct {
	Activation AtomicActivation
	Provenance string
	Reason     string
}

// validate requires evidence for a resolved permission and an explanation for
// prohibition; unresolved optional permission can remain in an analysis plan.
// Rules: rules/platform/platform_model.md — §§ 9, 14, 21;
// rules/concurrency/atomics.md — § 37A(11)-(12).
func (p AtomicProfilePolicy) validate() error {
	switch p.Activation {
	case AtomicActivationUnknown:
		return nil
	case AtomicEnabled, AtomicDisabled, AtomicNotApplicable:
		if strings.TrimSpace(p.Provenance) == "" {
			return fmt.Errorf("resolved atomic profile activation requires producer provenance")
		}
		if p.Activation == AtomicDisabled && strings.TrimSpace(p.Reason) == "" {
			return fmt.Errorf("disabled atomic profile activation requires a reason")
		}
		return nil
	default:
		return fmt.Errorf("invalid atomic profile activation %q", p.Activation)
	}
}

// AtomicCapabilityRule derives concrete context facts from one verified
// representation/operation proof. Fact retains the exact carrier, layout,
// orders, category and required guarantees. CoveredRefinements explicitly lists
// every context refinement covered by that proof; nil means Fact's exact key
// only. This is a finite proof domain, not a wildcard or consumer-side heuristic.
// Different vectors, carriers or operations need their own producer evidence.
// A restricted vector may have a separate Unsupported rule; absent vectors stay
// Unknown. Conflicting/duplicate derivations are rejected, not ordered by input.
// Rules: rules/concurrency/atomics.md — § 37A(6), (8)-(11);
// rules/platform/target_profiles.md — § 19.
type AtomicCapabilityRule struct {
	Fact               AtomicCapability
	CoveredRefinements []AtomicContextRefinement
}

// WithAtomicCapabilityRules expands target-verified finite proof domains into
// the canonical exact-key table. No callbacks, host data or mutable producer
// objects survive freezing. Every derived key and its implementation/context
// guarantees pass the same validation as a directly provided concrete fact.
// Rules: rules/concurrency/atomics.md — § 37A(10)-(11), (13)-(14).
func (p ResolvedScalarPlan) WithAtomicCapabilityRules(policy AtomicProfilePolicy, rules []AtomicCapabilityRule) (ResolvedScalarPlan, error) {
	var facts []AtomicCapability
	for _, rule := range rules {
		if err := rule.Fact.validate(); err != nil {
			return ResolvedScalarPlan{}, fmt.Errorf("invalid atomic producer rule: %w", err)
		}
		if len(rule.CoveredRefinements) == 0 {
			facts = append(facts, rule.Fact)
			continue
		}
		if rule.Fact.Key.Refinement != (AtomicContextRefinement{}) {
			return ResolvedScalarPlan{}, fmt.Errorf("atomic refinement derivation requires a category-wide base key")
		}
		for _, refinement := range rule.CoveredRefinements {
			fact := rule.Fact
			fact.Key.Refinement = refinement
			facts = append(facts, fact)
		}
	}
	return p.withAtomicCapabilities(policy, facts)
}
