package target

import (
	"fmt"
	"sort"

	"sec/internal/layout"
)

// Target-profile families (rules/platform/target_profiles.md § 4).
const (
	FamilyHosted    = "hosted"
	FamilyRTOS      = "rtos"
	FamilyBareMetal = "baremetal"
)

// AllocationProvider is one entry of a target's verified allocation-provider
// catalog: a canonical identity, the allocation modes it can serve, whether
// it is the target's declared default, and its provenance. A manifest can
// only select an entry; it cannot add one.
// Rules: rules/platform/target_profiles.md — § 41(4)-(5), (7).
type AllocationProvider struct {
	ID         string
	Modes      []string
	Default    bool
	Provenance string
}

// allocationProviderCatalogs holds the verified providers per LLVM triple.
// No target declares a provider yet: provider data must come with
// provenance and capability evidence, so every catalog is empty and an
// explicit hosted or embedded-arena mode is rejected until it is supplied.
// Rules: rules/platform/target_profiles.md — § 41(4); MD-139 correction § 3(4).
var allocationProviderCatalogs = map[string][]AllocationProvider{}

// ProfileFamily maps the definition's transitional registry profile label to
// its canonical family: hosted → hosted, rtos → rtos, freestanding →
// baremetal. Any other label fails closed.
// Rules: rules/platform/target_profiles.md — § 41(1).
func (definition Definition) ProfileFamily() (string, error) {
	switch definition.Profile {
	case "hosted":
		return FamilyHosted, nil
	case "rtos":
		return FamilyRTOS, nil
	case "freestanding":
		return FamilyBareMetal, nil
	}
	return "", fmt.Errorf("target %s/%s has registry profile %q without a target-profile family mapping", definition.OS, definition.Arch, definition.Profile)
}

// AllocationProviders returns the target's verified provider catalog.
// Rules: rules/platform/target_profiles.md — § 41(4).
func (definition Definition) AllocationProviders() []AllocationProvider {
	return append([]AllocationProvider(nil), allocationProviderCatalogs[definition.LLVMTriple]...)
}

// ResolvedTargetProfile is the frozen target-profile and allocation result of
// one Target–Variant plan (target_profiles.md § 41(7)). ProviderIdentity is
// empty when Absent. Capability facts beyond availability, activation and
// context kind are owned by the provider entry once providers exist.
type ResolvedTargetProfile struct {
	TargetProfileIdentity  string
	BaseProfileFamily      string
	AllocationPolicy       string
	PolicyFromBaseline     bool
	AllocationAvailability layout.AllocationAvailability
	AllocationActivation   layout.AllocationActivation
	AllocationContextKind  layout.AllocationContextProvider
	ProviderIdentity       string
	ProviderProvenance     string
}

// ResolveTargetProfile freezes the target profile and allocation policy of a
// definition under a manifest selection (empty strings mean "not selected").
//
// The selected profile must be the target's own family: reclassifying a
// target is a configuration error. Without a mode the family baseline
// applies (Hosted enables only a supported provider, RTOS/BareMetal start
// disabled) and nothing unknown becomes supported. noalloc is complete with
// provider Absent and rejects a provider. hosted and embedded-arena need a
// provider from the target's verified catalog that serves the mode: the
// named one, or else a unique declared default; none or several defaults is
// an error. A provider without a mode is checked but enables nothing.
// Rules: rules/platform/target_profiles.md — §§ 41(1)-(7); rules/memory/allocation.md — § 28A;
// MD-139 correction §§ 2-5.
func ResolveTargetProfile(definition Definition, profile, mode, provider string) (ResolvedTargetProfile, error) {
	family, err := definition.ProfileFamily()
	if err != nil {
		return ResolvedTargetProfile{}, err
	}
	if profile != "" && profile != family {
		return ResolvedTargetProfile{}, fmt.Errorf("target %s/%s is a %s target; profile %q would reclassify it", definition.OS, definition.Arch, family, profile)
	}
	resolved := ResolvedTargetProfile{
		TargetProfileIdentity: family, BaseProfileFamily: family,
		AllocationAvailability: layout.AllocationAvailabilityUnknown,
		AllocationActivation:   layout.AllocationDisabled,
		AllocationContextKind:  layout.AllocationNoContext,
	}
	if mode == "" {
		resolved.PolicyFromBaseline = true
		resolved.AllocationPolicy = "noalloc"
		if family == FamilyHosted {
			resolved.AllocationPolicy = "hosted"
		}
		if provider != "" {
			if _, err := lookupProvider(definition, provider, ""); err != nil {
				return ResolvedTargetProfile{}, err
			}
		}
		if family == FamilyHosted {
			if selected, err := lookupProvider(definition, "", "hosted"); err == nil {
				resolved.activate(selected, layout.AllocationCompilerContext)
			}
		}
		return resolved, nil
	}
	resolved.AllocationPolicy = mode
	switch mode {
	case "noalloc":
		if provider != "" {
			return ResolvedTargetProfile{}, fmt.Errorf("provider %q contradicts allocation mode noalloc", provider)
		}
		return resolved, nil
	case "hosted", "embedded-arena":
		selected, err := lookupProvider(definition, provider, mode)
		if err != nil {
			return ResolvedTargetProfile{}, fmt.Errorf("allocation mode %s on %s: %w", mode, definition.LLVMTriple, err)
		}
		context := layout.AllocationCompilerContext
		if mode == "embedded-arena" {
			context = layout.AllocationSuppliedContext
		}
		resolved.activate(selected, context)
		return resolved, nil
	}
	return ResolvedTargetProfile{}, fmt.Errorf("allocation mode %q is not hosted, embedded-arena or noalloc", mode)
}

// activate records a verified provider as the supported, enabled context.
// Rules: rules/platform/target_profiles.md — § 41(7).
func (resolved *ResolvedTargetProfile) activate(provider AllocationProvider, context layout.AllocationContextProvider) {
	resolved.AllocationAvailability = layout.AllocationSupported
	resolved.AllocationActivation = layout.AllocationEnabled
	resolved.AllocationContextKind = context
	resolved.ProviderIdentity, resolved.ProviderProvenance = provider.ID, provider.Provenance
}

// lookupProvider finds the named provider, or the unique declared default
// serving mode when name is empty, in the target's verified catalog. An
// empty mode skips the mode check.
// Rules: rules/platform/target_profiles.md — § 41(4)-(5).
func lookupProvider(definition Definition, name, mode string) (AllocationProvider, error) {
	catalog := definition.AllocationProviders()
	if name != "" {
		for _, provider := range catalog {
			if provider.ID == name {
				if mode != "" && !providerServes(provider, mode) {
					return AllocationProvider{}, fmt.Errorf("provider %q does not serve allocation mode %s", name, mode)
				}
				return provider, nil
			}
		}
		return AllocationProvider{}, fmt.Errorf("provider %q is not in the verified provider catalog of %s (%d providers)", name, definition.LLVMTriple, len(catalog))
	}
	var defaults []string
	var selected AllocationProvider
	for _, provider := range catalog {
		if provider.Default && providerServes(provider, mode) {
			defaults = append(defaults, provider.ID)
			selected = provider
		}
	}
	sort.Strings(defaults)
	switch len(defaults) {
	case 1:
		return selected, nil
	case 0:
		return AllocationProvider{}, fmt.Errorf("no provider is named and the target declares no verified default provider")
	}
	return AllocationProvider{}, fmt.Errorf("several default providers %v serve the mode; name one", defaults)
}

// providerServes reports whether provider lists mode.
// Rules: rules/platform/target_profiles.md — § 41(8).
func providerServes(provider AllocationProvider, mode string) bool {
	for _, served := range provider.Modes {
		if served == mode {
			return true
		}
	}
	return false
}
