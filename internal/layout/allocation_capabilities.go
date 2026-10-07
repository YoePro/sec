package layout

// AllocationAvailability separates a facility's availability from whether an
// allocation profile enables its use. Unknown never supplies positive proof.
// Rules: rules/platform/target_profiles.md — §3; rules/memory/allocation.md — §22.
type AllocationAvailability string

const (
	AllocationSupported           AllocationAvailability = "supported"
	AllocationUnsupported         AllocationAvailability = "unsupported"
	AllocationAvailabilityUnknown AllocationAvailability = "unknown"
)

// AllocationActivationUnknown is permitted only in unresolved allocation
// facts, not a completely frozen ResolvedTargetProfile.
// Rules: rules/platform/target_profiles.md — §§3,23.
type AllocationActivation string

const (
	AllocationEnabled           AllocationActivation = "enabled"
	AllocationDisabled          AllocationActivation = "disabled"
	AllocationActivationUnknown AllocationActivation = "unknown"
)

type AllocationCapability struct {
	Availability AllocationAvailability
	Activation   AllocationActivation
}

// AllocationContextProvider describes the allocation profile's context source,
// never a concrete allocator, domain identity, capacity or ISR permission.
// Rules: rules/memory/allocation.md — §§5(2),(8),21,22(2),(5).
type AllocationContextProvider string

const (
	AllocationCompilerContext AllocationContextProvider = "compiler-managed"
	AllocationSuppliedContext AllocationContextProvider = "fixed-or-program/platform-supplied"
	AllocationNoContext       AllocationContextProvider = "none"
	AllocationUnknownContext  AllocationContextProvider = "unknown"
)

// AllocationCapabilities is the canonical allocation subset shared by target
// plans and frontend/tooling consumers. It is not the complete target-profile
// model: allocator growth, finite capacity, ISR restrictions and physical
// layout guarantees require their own resolved facts. In particular disabling
// allocation does not imply an allocator is physically unsupported.
// Rules: rules/memory/allocation.md — §§22,24(4),29(1);
// rules/platform/target_profiles.md — §§3,7,23,29.
type AllocationCapabilities struct {
	Profile                     string
	ProfileKnown                bool
	DynamicAllocation           AllocationCapability
	FixedOrSuppliedArenaContext AllocationCapability
	ContextProvider             AllocationContextProvider
}

// ResolveAllocationCapabilities resolves only canonical allocation profiles.
// Hosted supplies the compiler-managed context; embedded-arena supplies the
// fixed/program/platform Arena context; noalloc disables dynamic allocation.
// An absent/derived/unmapped profile stays unknown, including freestanding
// until MD-034 is resolved. No OS/family spelling can enable allocation.
// Rules: rules/memory/allocation.md — §22(1)-(6);
// rules/platform/target_profiles.md — §§3,7,24,29,31; missing-decisions.yaml — MD-034.
func ResolveAllocationCapabilities(profile string) AllocationCapabilities {
	unknown := AllocationCapability{AllocationAvailabilityUnknown, AllocationActivationUnknown}
	facts := AllocationCapabilities{Profile: profile, DynamicAllocation: unknown, FixedOrSuppliedArenaContext: unknown, ContextProvider: AllocationUnknownContext}
	switch profile {
	case "hosted":
		facts.ProfileKnown = true
		facts.DynamicAllocation = AllocationCapability{AllocationSupported, AllocationEnabled}
		facts.ContextProvider = AllocationCompilerContext
	case "embedded-arena":
		facts.ProfileKnown = true
		facts.DynamicAllocation = AllocationCapability{AllocationSupported, AllocationEnabled}
		facts.FixedOrSuppliedArenaContext = AllocationCapability{AllocationSupported, AllocationEnabled}
		facts.ContextProvider = AllocationSuppliedContext
	case "noalloc":
		facts.ProfileKnown = true
		facts.DynamicAllocation.Activation = AllocationDisabled
		facts.ContextProvider = AllocationNoContext
	}
	return facts
}

// AllocationCapabilities projects the selected plan's profile without choosing
// an allocator or guessing a capability from OS, CPU, ABI or pointer width.
// Rules: rules/memory/allocation.md — §§22,26(2)-(3);
// rules/platform/target_profiles.md — §§2,31.
func (p ResolvedScalarPlan) AllocationCapabilities() AllocationCapabilities {
	return ResolveAllocationCapabilities(p.Profile)
}

// HasActiveArenaContext requires positive availability and activation. A
// denied or unresolved capability cannot become a materialization context.
// Rules: rules/memory/allocation.md — §§5(2),17(9),22(4);
// rules/platform/target_profiles.md — §§3,29.
func (f AllocationCapabilities) HasActiveArenaContext() bool {
	return f.ProfileKnown && f.DynamicAllocation.Availability == AllocationSupported && f.DynamicAllocation.Activation == AllocationEnabled &&
		(f.ContextProvider == AllocationCompilerContext || f.ContextProvider == AllocationSuppliedContext)
}
