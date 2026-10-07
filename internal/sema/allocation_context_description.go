package sema

import "fmt"

// AllocationContextDescription renders semantic selection evidence consistently
// for CLI and LSP. Internal domain tokens are analysis-local identities; a newly
// created Arena is kept distinct from its still-unresolved backing context.
// Rules: rules/memory/allocation.md — §§5(8),29(1),(3); rules/memory/arena.md — §§4.2,4.3,67(4).
func AllocationContextDescription(fact AllocationContextFact) string {
	if fact.ContextKnown {
		availability := "available"
		if !fact.Context.Available {
			availability = "unavailable"
		}
		profile := fact.Context.Profile
		if profile == "" {
			profile = "unresolved"
		}
		return fmt.Sprintf("implicit allocation context: %s; profile %s; origin %s; domain identity unresolved", availability, profile, fact.Context.Origin)
	}
	if fact.SelectedDomain != "" {
		return fmt.Sprintf("explicit Arena allocation domain: %s (receiver %s)", fact.SelectedDomain, fact.Arena)
	}
	if fact.CreatedDomain != "" {
		return fmt.Sprintf("created Arena domain: %s; backing allocation context unresolved", fact.CreatedDomain)
	}
	return "allocation context/domain unresolved"
}
