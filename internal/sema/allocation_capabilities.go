package sema

import "sec/internal/layout"

// AllocationCapabilities returns compiler-owned allocation profile facts.
// Unbound analysis retains its existing hosted source-analysis default; an
// explicitly supplied empty/unknown profile does not inherit that default.
// Rules: rules/memory/allocation.md — §§22,24(4),29(1);
// rules/platform/target_profiles.md — §§3,31.
func (a *Analyzer) AllocationCapabilities() layout.AllocationCapabilities {
	profile := a.targetProfile
	if !a.allocationProfileSelected {
		profile = "hosted"
	}
	return layout.ResolveAllocationCapabilities(profile)
}

// SetAllocationProfile supplies an independently resolved allocation profile
// when scalar representation is target-independent (e.g. same allocation
// profile across distinct width variants). Empty means explicitly unresolved.
// Callers must resolve current canonical plan inputs before analysis.
// Rules: rules/platform/target_profiles.md — §§2,23,31-32;
// rules/memory/allocation.md — §§22,29(1),(5).
func (a *Analyzer) SetAllocationProfile(profile string) {
	a.targetProfile = profile
	a.allocationProfileSelected = true
}
