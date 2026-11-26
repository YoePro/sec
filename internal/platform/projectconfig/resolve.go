package projectconfig

import (
	"fmt"
	"sec/internal/platform/cbindingplan"
	"sec/internal/platform/target"
	"sec/internal/project"
)

type Resolution struct {
	Selection   ResolutionSelection `json:"selection"`
	Binding     *cbindingplan.Plan  `json:"binding"`
	Diagnostics []Diagnostic        `json:"diagnostics"`
}
type ResolutionSelection struct {
	Target  string         `json:"target"`
	Variant string         `json:"variant"`
	Facts   *ResolvedFacts `json:"facts"`
}
type ResolvedFacts struct {
	Selection project.PlanSelection        `json:"selection"`
	Profile   target.ResolvedTargetProfile `json:"profile"`
	OS        string                       `json:"os"`
	Arch      string                       `json:"arch"`
	Triple    string                       `json:"triple"`
	ABI       string                       `json:"abi"`
}

// Resolve returns the same selected profile and binding plan consumed by the
// compiler/LSP, with scope provenance and canonical binding fingerprints.
// No host fallback, provider record or capability is manufactured.
// Rules: rules/platform/target_profiles.md §41; MD-021 correction §8.7;
// harness design notes §§4(3),5(7),7(3).
func (w *Workspace) Resolve(targetName, variant string) (Resolution, error) {
	out := Resolution{Selection: ResolutionSelection{Target: targetName, Variant: variant}, Diagnostics: []Diagnostic{}}
	if targetName == "" || variant == "" {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{"workspace.unknown-plan", "explicit declared Target and Variant are required"})
		return out, nil
	}
	data, err := w.root.ReadFile(File)
	if err != nil {
		return out, err
	}
	resolutions, diagnostics := Resolve(string(data))
	if len(diagnostics) > 0 {
		out.Diagnostics = diagnostics
		return out, nil
	}
	for _, entry := range resolutions {
		if entry.Target != targetName || entry.Variant != variant {
			continue
		}
		out.Selection.Facts = &ResolvedFacts{entry.Selection, entry.Profile, entry.Platform.OS, entry.Platform.Arch, entry.Triple, entry.ABI}
		plan, problems := cbindingplan.ResolveSnapshotInRoot(w.root, w.identity, variant, entry.Platform, data)
		out.Binding = plan
		for _, problem := range problems {
			out.Diagnostics = append(out.Diagnostics, Diagnostic{"workspace.binding-resolution", problem})
		}
		return out, nil
	}
	out.Diagnostics = append(out.Diagnostics, Diagnostic{"workspace.unknown-plan", fmt.Sprintf("Target %q Variant %q is not a declared plan", targetName, variant)})
	return out, nil
}
