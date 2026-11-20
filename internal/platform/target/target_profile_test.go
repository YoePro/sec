package target

import (
	"strings"
	"testing"

	"sec/internal/layout"
)

// MD-139 § 2(3): every registered target with a registry label maps it to
// exactly one family; a planned target without a label (rtems, zephyr) fails
// closed instead of receiving a guessed family.
func TestEveryTargetHasProfileFamily(t *testing.T) {
	for _, definition := range Definitions() {
		family, err := definition.ProfileFamily()
		if definition.Profile == "" {
			if err == nil || definition.Status != Planned {
				t.Errorf("%s/%s without a profile label resolved to %q (status %s)", definition.OS, definition.Arch, family, definition.Status)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s/%s: %v", definition.OS, definition.Arch, err)
		}
		if definition.Profile == "freestanding" && family != FamilyBareMetal {
			t.Errorf("%s/%s freestanding maps to %q", definition.OS, definition.Arch, family)
		}
	}
}

func targetByOSArch(t *testing.T, os, arch string) Definition {
	t.Helper()
	for _, definition := range Definitions() {
		if definition.OS == os && definition.Arch == arch {
			return definition
		}
	}
	t.Fatalf("no target %s/%s", os, arch)
	return Definition{}
}

// withProviders installs a synthetic catalog for one test.
func withProviders(t *testing.T, triple string, providers []AllocationProvider) {
	t.Helper()
	previous, had := allocationProviderCatalogs[triple]
	allocationProviderCatalogs[triple] = providers
	t.Cleanup(func() {
		if had {
			allocationProviderCatalogs[triple] = previous
		} else {
			delete(allocationProviderCatalogs, triple)
		}
	})
}

// MD-139 § 8 conformance matrix, rows that need no real provider data.
func TestResolveTargetProfileBaselines(t *testing.T) {
	linux := targetByOSArch(t, "linux", "amd64")
	hosted, err := ResolveTargetProfile(linux, "", "", "")
	if err != nil || hosted.BaseProfileFamily != FamilyHosted || !hosted.PolicyFromBaseline ||
		hosted.AllocationAvailability != layout.AllocationAvailabilityUnknown || hosted.AllocationActivation != layout.AllocationDisabled || hosted.ProviderIdentity != "" {
		t.Fatalf("hosted baseline without a provider = %+v %v", hosted, err)
	}
	board := targetByOSArch(t, "baremetal", "cortex-m4")
	bare, err := ResolveTargetProfile(board, "baremetal", "", "")
	if err != nil || bare.BaseProfileFamily != FamilyBareMetal || bare.AllocationPolicy != "noalloc" || bare.AllocationActivation != layout.AllocationDisabled {
		t.Fatalf("baremetal baseline = %+v %v", bare, err)
	}
	noalloc, err := ResolveTargetProfile(linux, "hosted", "noalloc", "")
	if err != nil || noalloc.AllocationPolicy != "noalloc" || noalloc.AllocationContextKind != layout.AllocationNoContext || noalloc.ProviderIdentity != "" {
		t.Fatalf("explicit noalloc = %+v %v", noalloc, err)
	}
	for name, check := range map[string]struct {
		definition              Definition
		profile, mode, provider string
		want                    string
	}{
		"reclassify":                {linux, "baremetal", "", "", "would reclassify"},
		"noalloc with provider":     {linux, "", "noalloc", "heap", "contradicts"},
		"embedded-arena unknown":    {board, "", "embedded-arena", "", "no verified default provider"},
		"unknown provider":          {board, "", "embedded-arena", "board-primary-arena", "not in the verified provider catalog"},
		"provider without mode":     {linux, "", "", "heap", "not in the verified provider catalog"},
		"explicit hosted, no proof": {linux, "", "hosted", "", "no verified default provider"},
	} {
		if _, err := ResolveTargetProfile(check.definition, check.profile, check.mode, check.provider); err == nil || !strings.Contains(err.Error(), check.want) {
			t.Errorf("%s: %v, want %q", name, err, check.want)
		}
	}
}

// MD-139 § 3(5)-(7), § 8: a verified provider activates its mode; a unique
// default is used; two defaults are ambiguous; a provider serving another
// mode is rejected.
func TestResolveTargetProfileWithProviders(t *testing.T) {
	linux := targetByOSArch(t, "linux", "amd64")
	withProviders(t, linux.LLVMTriple, []AllocationProvider{
		{ID: "system-heap", Modes: []string{"hosted"}, Default: true, Provenance: "test"},
		{ID: "static-arena", Modes: []string{"embedded-arena"}, Provenance: "test"},
	})
	baseline, err := ResolveTargetProfile(linux, "", "", "")
	if err != nil || baseline.AllocationActivation != layout.AllocationEnabled || baseline.ProviderIdentity != "system-heap" || baseline.AllocationContextKind != layout.AllocationCompilerContext {
		t.Fatalf("hosted baseline with a default provider = %+v %v", baseline, err)
	}
	arena, err := ResolveTargetProfile(linux, "", "embedded-arena", "static-arena")
	if err != nil || arena.AllocationContextKind != layout.AllocationSuppliedContext || arena.ProviderIdentity != "static-arena" {
		t.Fatalf("named embedded-arena provider = %+v %v", arena, err)
	}
	if _, err := ResolveTargetProfile(linux, "", "hosted", "static-arena"); err == nil || !strings.Contains(err.Error(), "does not serve") {
		t.Fatalf("provider for another mode: %v", err)
	}
	withProviders(t, linux.LLVMTriple, []AllocationProvider{
		{ID: "a", Modes: []string{"hosted"}, Default: true}, {ID: "b", Modes: []string{"hosted"}, Default: true},
	})
	if _, err := ResolveTargetProfile(linux, "", "hosted", ""); err == nil || !strings.Contains(err.Error(), "several default providers") {
		t.Fatalf("two defaults: %v", err)
	}
}
