package project

import (
	"fmt"
	"sort"
)

// AllocationMode is a canonical allocation policy selectable in a manifest
// `[*.allocation]` table (MD-139).
type AllocationMode string

const (
	AllocationModeHosted        AllocationMode = "hosted"
	AllocationModeEmbeddedArena AllocationMode = "embedded-arena"
	AllocationModeNoalloc       AllocationMode = "noalloc"
)

// Target-profile families selectable through a `profile` field (MD-139).
const (
	TargetProfileHosted    = "hosted"
	TargetProfileRTOS      = "rtos"
	TargetProfileBareMetal = "baremetal"
)

// PlanSelection is the project-level selection of one Target–Variant plan:
// the requested target profile and allocation policy after scope precedence,
// each remembering the most specific scope that set it. Empty fields mean
// "not selected"; the registry default and family baseline then apply.
type PlanSelection struct {
	Target, Variant string
	Profile         string
	ProfileScope    string
	AllocationMode  AllocationMode
	ModeScope       string
	Provider        string
	ProviderScope   string
}

// ResolvePlanSelection reads the target-profile and allocation selections of
// one Target–Variant pair from a manifest. Scopes apply from least to most
// specific: `[variant.V]`, `[target.T]`, `[target.T.variant.V]`; each key is
// replaced independently. Every visited table is validated: an unknown
// profile family, an unknown allocation key or mode, an empty provider, and
// provider with noalloc (in one table or after merging) are errors. An empty
// target or variant name skips that scope.
// Rules: rules/projects/projects.md — §§ 19(11)-(15), 25(2);
// rules/platform/target_profiles.md — § 41(1)-(6).
func ResolvePlanSelection(text, target, variant string) (PlanSelection, []error) {
	selection := PlanSelection{Target: target, Variant: variant}
	doc, err := ParseManifestDocument(text)
	if err != nil {
		return selection, []error{err}
	}
	var errs []error
	scopes := [][]string{}
	if variant != "" {
		scopes = append(scopes, []string{"variant", variant})
	}
	if target != "" {
		scopes = append(scopes, []string{"target", target})
		if variant != "" {
			scopes = append(scopes, []string{"target", target, "variant", variant})
		}
	}
	for _, path := range scopes {
		table, err := doc.Table(path...)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if table == nil {
			continue
		}
		scope := joinScope(path)
		if value, found := table["profile"]; found {
			profile, ok := value.(string)
			if !ok || !knownTargetProfile(profile) {
				errs = append(errs, fmt.Errorf("[%s].profile = %v is not a target profile (hosted, rtos or baremetal); build profiles belong in [build].profile", scope, value))
			} else {
				selection.Profile, selection.ProfileScope = profile, scope
			}
		}
		allocation, err := doc.Table(append(append([]string(nil), path...), "allocation")...)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if allocation == nil {
			continue
		}
		errs = append(errs, applyAllocationTable(&selection, scope+".allocation", allocation)...)
	}
	if selection.AllocationMode == AllocationModeNoalloc && selection.Provider != "" {
		errs = append(errs, fmt.Errorf("provider %q from [%s] contradicts mode \"noalloc\" from [%s]", selection.Provider, selection.ProviderScope, selection.ModeScope))
	}
	return selection, errs
}

// applyAllocationTable validates one `[*.allocation]` table and applies its
// keys over the less specific selection.
// Rules: rules/projects/projects.md — § 19(12)-(13); rules/platform/target_profiles.md — § 41(3)-(4).
func applyAllocationTable(selection *PlanSelection, scope string, table map[string]any) []error {
	var errs []error
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key != "mode" && key != "provider" {
			errs = append(errs, fmt.Errorf("[%s] has unknown key %q; only mode and provider are allowed", scope, key))
		}
	}
	mode, hasMode := table["mode"]
	if hasMode {
		text, ok := mode.(string)
		switch AllocationMode(text) {
		case AllocationModeHosted, AllocationModeEmbeddedArena, AllocationModeNoalloc:
		default:
			ok = false
		}
		if !ok {
			errs = append(errs, fmt.Errorf("[%s].mode = %v is not hosted, embedded-arena or noalloc", scope, mode))
			hasMode = false
		} else {
			selection.AllocationMode, selection.ModeScope = AllocationMode(text), scope
		}
	}
	if provider, found := table["provider"]; found {
		text, ok := provider.(string)
		if !ok || text == "" {
			errs = append(errs, fmt.Errorf("[%s].provider must be a non-empty provider identity", scope))
		} else {
			if hasMode && AllocationMode(table["mode"].(string)) == AllocationModeNoalloc {
				errs = append(errs, fmt.Errorf("[%s] names provider %q together with mode \"noalloc\"", scope, text))
			}
			selection.Provider, selection.ProviderScope = text, scope
		}
	}
	return errs
}

// knownTargetProfile accepts the canonical target-profile families. No
// derived-profile registry exists yet, so any other name is unknown.
// Rules: rules/platform/target_profiles.md — § 41(1)-(2).
func knownTargetProfile(profile string) bool {
	return profile == TargetProfileHosted || profile == TargetProfileRTOS || profile == TargetProfileBareMetal
}

// joinScope spells a table path such as variant.board or target.app.variant.board.
// Rules: rules/projects/projects.md — § 19(14).
func joinScope(path []string) string {
	scope := path[0]
	for _, part := range path[1:] {
		scope += "." + part
	}
	return scope
}
