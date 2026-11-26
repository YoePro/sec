// Package projectconfig edits MD-139 selections through the canonical project
// parser and target-profile resolver, without inventing provider capabilities.
package projectconfig

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"sec/internal/configedit"
	"sec/internal/platform/target"
	"sec/internal/project"
)

const File = ".sec/sec.toml"

// Operation selects an existing Variant, Target or Target-specific Variant.
// Nil fields preserve the local value; an empty string removes the local key
// and restores ordinary inheritance. Names never become filesystem paths.
type Operation struct {
	Target   string  `json:"target,omitempty"`
	Variant  string  `json:"variant,omitempty"`
	Profile  *string `json:"profile,omitempty"`
	Mode     *string `json:"mode,omitempty"`
	Provider *string `json:"provider,omitempty"`
}

type Diagnostic struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type EditResult struct {
	Text        string       `json:"-"`
	Changed     bool         `json:"changed"`
	Rejected    bool         `json:"rejected"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Resolve validates every reusable Variant and declared Target-Variant pair,
// sharing the same precedence and target catalog as CLI/LSP pre-analysis.
// It publishes no supported allocation capability from a missing provider.
// Rules: rules/projects/projects.md §§19(11)-(16),25(2); rules/platform/target_profiles.md §41.
func Resolve(text string) ([]ResolutionEntry, []Diagnostic) {
	fail := func(err error) ([]ResolutionEntry, []Diagnostic) {
		return nil, []Diagnostic{{"project-config.invalid", err.Error()}}
	}
	if _, errors := project.ParseManifest(filepath.Join(string(filepath.Separator), "workspace", filepath.FromSlash(File)), text); len(errors) > 0 {
		return fail(fmt.Errorf("%v", errors))
	}
	doc, err := project.ParseManifestDocument(text)
	if err != nil {
		return fail(err)
	}
	variants, err := doc.Table("variant")
	if err != nil {
		return fail(err)
	}
	targets, err := doc.Table("target")
	if err != nil {
		return fail(err)
	}
	type pair struct{ target, variant string }
	var pairs []pair
	for name := range variants {
		pairs = append(pairs, pair{"", name})
	}
	targetNames := make([]string, 0, len(targets))
	for name := range targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		value := targets[name]
		table, ok := value.(map[string]any)
		if !ok {
			return fail(fmt.Errorf("target %q must be a table", name))
		}
		selected := map[string]bool{}
		if names, present := table["variants"]; present {
			list, ok := names.([]any)
			if !ok {
				return fail(fmt.Errorf("target %q variants must be a string array", name))
			}
			for _, value := range list {
				v, ok := value.(string)
				if !ok {
					return fail(fmt.Errorf("target %q variants must be a string array", name))
				}
				if selected[v] {
					return fail(fmt.Errorf("target %q repeats variant %q", name, v))
				}
				selected[v] = true
			}
		} else {
			for v := range variants {
				selected[v] = true
			}
		}
		inner, err := doc.Table("target", name, "variant")
		if err != nil {
			return fail(err)
		}
		for v := range inner {
			selected[v] = true
		}
		if len(selected) == 0 {
			if _, errs := project.ResolvePlanSelection(text, name, ""); len(errs) > 0 {
				return fail(errs[0])
			}
		}
		for v := range selected {
			pairs = append(pairs, pair{name, v})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].target+"\x00"+pairs[i].variant < pairs[j].target+"\x00"+pairs[j].variant
	})
	resolutions := []ResolutionEntry{}
	problems := []Diagnostic{}
	for _, p := range pairs {
		selection, errs := project.ResolvePlanSelection(text, p.target, p.variant)
		selectors, more := project.ReadVariantCBindings(text, p.variant)
		errs = append(errs, more...)
		if len(errs) > 0 {
			for _, err := range errs {
				problems = append(problems, Diagnostic{"manifest.plan-selection", fmt.Sprintf("Target %s Variant %s: %v", p.target, p.variant, err)})
			}
			continue
		}
		platform, ok := target.Parse(selectors.OS + "-" + selectors.Arch)
		if !ok {
			problems = append(problems, Diagnostic{"manifest.plan-selection", fmt.Sprintf("Variant %s: unknown target %s/%s", p.variant, selectors.OS, selectors.Arch)})
			continue
		}
		definition, ok := target.Find(platform)
		if !ok {
			problems = append(problems, Diagnostic{"manifest.plan-selection", fmt.Sprintf("Variant %s: target is not registered", p.variant)})
			continue
		}
		profile, err := target.ResolveTargetProfile(definition, selection.Profile, string(selection.AllocationMode), selection.Provider)
		if err != nil {
			problems = append(problems, Diagnostic{"manifest.plan-selection", fmt.Sprintf("Target %s Variant %s: %v", p.target, p.variant, err)})
			continue
		}
		resolutions = append(resolutions, ResolutionEntry{p.target, p.variant, platform, definition.LLVMTriple, definition.ABI, selection, profile})
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return resolutions, problems
}

// ResolutionEntry preserves scope provenance alongside the canonical frozen profile.
type ResolutionEntry struct {
	Target    string                       `json:"target"`
	Variant   string                       `json:"variant"`
	Platform  target.Target                `json:"platform"`
	Triple    string                       `json:"triple"`
	ABI       string                       `json:"abi"`
	Selection project.PlanSelection        `json:"selection"`
	Profile   target.ResolvedTargetProfile `json:"profile"`
}

// Apply is the sole typed profile/allocation editor. It validates the exact
// expected parsed model after a minimal byte splice; unrelated data and comments
// survive. Rejected edits return the original bytes, including bad input.
// Rules: rules/projects/projects.md §§5(2),(4)-(6),19(11)-(15); harness notes §§3,5.
func Apply(text string, op Operation) EditResult {
	result := EditResult{Text: text, Diagnostics: []Diagnostic{}}
	reject := func(err error) EditResult {
		result.Rejected = true
		result.Diagnostics = []Diagnostic{{"project-config.invalid-edit", err.Error()}}
		return result
	}
	doc, err := project.ParseManifestDocument(text)
	if err != nil {
		return reject(err)
	}
	if op.Target == "" && op.Variant == "" {
		return reject(fmt.Errorf("an existing target or variant is required"))
	}
	if op.Profile == nil && op.Mode == nil && op.Provider == nil {
		return reject(fmt.Errorf("at least one profile, mode or provider field is required"))
	}
	var path []string
	if op.Target != "" {
		if !project.ValidProjectKey(op.Target) {
			return reject(fmt.Errorf("invalid target name"))
		}
		table, err := doc.Table("target", op.Target)
		if err != nil || table == nil {
			return reject(fmt.Errorf("unknown target %q", op.Target))
		}
		path = []string{"target", op.Target}
	}
	if op.Variant != "" {
		if !project.ValidProjectKey(op.Variant) {
			return reject(fmt.Errorf("invalid variant name"))
		}
		table, err := doc.Table("variant", op.Variant)
		if err != nil || table == nil {
			return reject(fmt.Errorf("unknown variant %q", op.Variant))
		}
		path = append(path, "variant", op.Variant)
	}
	expected, _ := project.ParseManifestDocument(text)
	edited := text
	for _, field := range []struct {
		key        string
		value      *string
		allocation bool
	}{{"profile", op.Profile, false}, {"mode", op.Mode, true}, {"provider", op.Provider, true}} {
		if field.value == nil {
			continue
		}
		table := append([]string{}, path...)
		if field.allocation {
			table = append(table, "allocation")
		}

		existing, err := expected.Table(table...)
		if err != nil {
			return reject(err)
		}
		if *field.value == "" {
			if existing == nil {
				continue
			}
			if _, found := existing[field.key]; !found {
				continue
			}
		} else if value, ok := existing[field.key].(string); ok && value == *field.value {
			continue
		}
		model := expected.Values
		for _, part := range table {
			if model[part] == nil {
				model[part] = map[string]any{}
			}
			next, ok := model[part].(map[string]any)
			if !ok {
				return reject(fmt.Errorf("selection scope is not a table"))
			}
			model = next
		}
		scanner, err := configedit.Scan(edited)
		if err != nil {
			return reject(err)
		}
		if *field.value == "" {
			delete(model, field.key)
			edited, err = scanner.RemoveKey(table, field.key)
		} else {
			model[field.key] = *field.value
			edited, err = scanner.SetValue(table, field.key, configedit.String(*field.value))
		}
		if err != nil {
			return reject(err)
		}
	}
	got, err := project.ParseManifestDocument(edited)
	if err != nil {
		return reject(err)
	}
	if !reflect.DeepEqual(got.Values, expected.Values) {
		return reject(fmt.Errorf("file layout cannot be edited without changing unrelated fields; use explicit tables"))
	}
	if _, problems := Resolve(edited); len(problems) > 0 {
		result.Rejected = true
		result.Diagnostics = problems
		return result
	}
	result.Text = edited
	result.Changed = edited != text
	return result
}
