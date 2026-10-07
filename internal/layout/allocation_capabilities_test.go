package layout

import "testing"

// TestAllocationCapabilityProfiles checks separate availability and activation,
// canonical context sources and conservative unresolved profiles. Architecture
// names and scalar widths supply no allocator/domain/capacity proof.
// Rules: rules/memory/allocation.md — §22;
// rules/platform/target_profiles.md — §§2,3,7,23-24,31.
func TestAllocationCapabilityProfiles(t *testing.T) {
	for _, test := range []struct {
		profile       string
		known, active bool
		availability  AllocationAvailability
		activation    AllocationActivation
		provider      AllocationContextProvider
		supplied      bool
	}{
		{"hosted", true, true, AllocationSupported, AllocationEnabled, AllocationCompilerContext, false},
		{"embedded-arena", true, true, AllocationSupported, AllocationEnabled, AllocationSuppliedContext, true},
		{"noalloc", true, false, AllocationAvailabilityUnknown, AllocationDisabled, AllocationNoContext, false},
		{"freestanding", false, false, AllocationAvailabilityUnknown, AllocationActivationUnknown, AllocationUnknownContext, false},
		{"", false, false, AllocationAvailabilityUnknown, AllocationActivationUnknown, AllocationUnknownContext, false},
		{"Hosted", false, false, AllocationAvailabilityUnknown, AllocationActivationUnknown, AllocationUnknownContext, false},
		{"custom", false, false, AllocationAvailabilityUnknown, AllocationActivationUnknown, AllocationUnknownContext, false},
	} {
		t.Run(test.profile, func(t *testing.T) {
			got := ResolveAllocationCapabilities(test.profile)
			if got.Profile != test.profile || got.ProfileKnown != test.known || got.HasActiveArenaContext() != test.active || got.DynamicAllocation.Availability != test.availability || got.DynamicAllocation.Activation != test.activation || got.ContextProvider != test.provider {
				t.Fatal(got, test)
			}
			supplied := got.FixedOrSuppliedArenaContext == (AllocationCapability{AllocationSupported, AllocationEnabled})
			if supplied != test.supplied {
				t.Fatal("invented fixed/supplied Arena support", got)
			}
			for _, width := range []uint16{0, 32, 64} {
				plan := ResolvedScalarPlan{TargetOS: "arbitrary", TargetArch: "arbitrary", Profile: test.profile, PointerWidthBits: width}
				if plan.AllocationCapabilities() != got {
					t.Fatal("scalar shape changed allocation facts", plan)
				}
			}
			detached := got
			detached.Profile = "mutated"
			if ResolveAllocationCapabilities(test.profile) != got {
				t.Fatal("facts were not detached")
			}
		})
	}
	disabled := ResolveAllocationCapabilities("noalloc")
	disabled.DynamicAllocation.Availability = AllocationSupported
	if disabled.HasActiveArenaContext() {
		t.Fatal("physical availability overrode disabled policy")
	}
	unsupported := ResolveAllocationCapabilities("hosted")
	unsupported.DynamicAllocation.Availability = AllocationUnsupported
	if unsupported.HasActiveArenaContext() {
		t.Fatal("activation enabled unsupported allocation")
	}
	if (AllocationCapabilities{}).HasActiveArenaContext() {
		t.Fatal("zero facts enabled allocation")
	}
}
