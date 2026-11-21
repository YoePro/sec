package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
)

// ResolvedMutexTargetCapability retains the selected plan evidence for an
// accepted overload. Unknown is not permission for physical lowering or an
// interrupt/nonblocking-context proof; those require their own contracts.
// Rules: rules/concurrency/mutex.md §§27,51,59–60,63.
type ResolvedMutexTargetCapability struct {
	Capability                                     layout.MutexCapability
	TargetOS, TargetArch, Profile, PlanFingerprint string
}

// resolveMutexTargetCapability rejects a proven lack of the exact acquisition
// contract before publishing successful acquisition facts. Missing evidence is
// retained as Unknown, never converted to a runtime ContextError or LockError.
// Rules: rules/concurrency/mutex.md §§20–24,27(6),60,63;
// rules/platform/platform_model.md §§9,23.
func (a *Analyzer) resolveMutexTargetCapability(call *ast.CallExpression, mode layout.MutexAcquisition) bool {
	plan := a.mutexTargetPlan
	fact := layout.MutexCapability{Acquisition: mode, Availability: layout.MutexUnknown}
	if len(plan.Mutex.Facts()) != 0 {
		var err error
		fact, err = plan.MutexCapability(mode)
		if err != nil {
			a.addErrorAtTokenWithMetadata(call.Token, diagnostics.MutexTargetCapability, "Resolve the selected CompilationPlan again; mutex evidence cannot be reused across target/profile changes.", "invalid selected mutex capability: %s", err)
			return false
		}
	}
	if fact.Availability == layout.MutexUnsupported {
		a.addErrorAtTokenWithMetadata(call.Token, diagnostics.MutexTargetCapability,
			fmt.Sprintf("Select a target/profile with a conforming %s mutex contract. Evidence: %s. This is a compile-time capability failure, not a runtime lock error.", mode, fact.Provenance),
			"target %s/%s profile %s does not support %s mutex acquisition: %s", plan.TargetOS, plan.TargetArch, plan.Profile, mode, fact.Reason)
		return false
	}
	if a.resolvedMutexTargetCapabilities == nil {
		a.resolvedMutexTargetCapabilities = map[*ast.CallExpression]ResolvedMutexTargetCapability{}
	}
	a.resolvedMutexTargetCapabilities[call] = ResolvedMutexTargetCapability{Capability: fact, TargetOS: plan.TargetOS, TargetArch: plan.TargetArch, Profile: plan.Profile, PlanFingerprint: a.planFingerprints.Plan}
	return true
}

// ResolvedMutexTargetCapabilityOf reads retained target evidence without name
// resolution, querying a host, or manufacturing support for a missing producer.
// Rules: rules/concurrency/mutex.md §§59–63; rules/compiler/semantic_ir.md §122.
func (a *Analyzer) ResolvedMutexTargetCapabilityOf(call *ast.CallExpression) (ResolvedMutexTargetCapability, bool) {
	if a == nil || call == nil {
		return ResolvedMutexTargetCapability{}, false
	}
	fact, ok := a.resolvedMutexTargetCapabilities[call]
	return fact, ok
}
