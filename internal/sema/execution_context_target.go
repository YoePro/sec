package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
)

// ResolvedThreadCurrentCapability retains physical-thread target evidence for
// one canonical Thread.Current call, distinct from a logical-task requirement.
// Unknown does not authorize physical lowering or synthesize a thread identity.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/compiler/semantic_ir.md §122.
type ResolvedThreadCurrentCapability struct {
	Capability                                     layout.PhysicalThreadCapability
	TargetOS, TargetArch, Profile, PlanFingerprint string
}

// resolveThreadCurrentCapability rejects explicit target denial without a
// synchronous fallback or a TaskContext substitution. Language-only analysis
// retains missing evidence as Unknown; physical lowering needs its own proof.
// Rules: rules/concurrency/cancellation.md §§11(2–3),24;
// rules/platform/platform_model.md §§9,23.
func (a *Analyzer) resolveThreadCurrentCapability(call *ast.CallExpression) bool {
	plan := a.targetCapabilityPlan
	fact := plan.PhysicalThreads.Fact()
	if plan.PhysicalThreads.Defined() {
		var err error
		fact, err = plan.PhysicalThreadCapability()
		if err != nil {
			a.addErrorAtTokenWithMetadata(call.Token, diagnostics.PhysicalThreadCapability,
				"Resolve the selected CompilationPlan again; physical-thread evidence cannot be reused across target/profile changes.",
				"invalid selected physical-thread capability: %s", err)
			return false
		}
	}
	if fact.Availability == layout.PhysicalThreadUnsupported {
		a.addErrorAtTokenWithMetadata(call.Token, diagnostics.PhysicalThreadCapability,
			fmt.Sprintf("Choose a target/profile with a physical-thread identity contract. Evidence: %s. Thread.Current() cannot be replaced by Task.Current() or a fabricated host thread.", fact.Provenance),
			"Thread.Current() requires physical-thread capability; target %s/%s profile %s does not provide it: %s", plan.TargetOS, plan.TargetArch, plan.Profile, fact.Reason)
		return false
	}
	if a.resolvedThreadCurrentCapabilities == nil {
		a.resolvedThreadCurrentCapabilities = map[*ast.CallExpression]ResolvedThreadCurrentCapability{}
	}
	a.resolvedThreadCurrentCapabilities[call] = ResolvedThreadCurrentCapability{Capability: fact, TargetOS: plan.TargetOS, TargetArch: plan.TargetArch, Profile: plan.Profile, PlanFingerprint: a.planFingerprints.Plan}
	return true
}

// ResolvedThreadCurrentCapabilityOf returns retained immutable evidence without
// resolving names or deriving a physical capability from compiler-host facts.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/platform/platform_model.md §23.
func (a *Analyzer) ResolvedThreadCurrentCapabilityOf(call *ast.CallExpression) (ResolvedThreadCurrentCapability, bool) {
	if a == nil || call == nil {
		return ResolvedThreadCurrentCapability{}, false
	}
	fact, ok := a.resolvedThreadCurrentCapabilities[call]
	return fact, ok
}
