package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/project"
)

// initTarget extends the governing manifest and creates its entry module in one
// transaction. New Targets inherit the Project version and receive no counter.
// Rules: rules/projects/projects.md — §§6,17–19,39–45;
// rules/compiler/initialization.md — §§20,25–27; rules/tooling/testing.md — §5.
func initTarget(options initCommandOptions) error {
	plan, manifest, err := planInitTarget(options)
	if err != nil {
		return err
	}
	if err := applyInitTarget(plan, manifest, options.DryRun); err != nil {
		return err
	}
	if !options.DryRun {
		fmt.Printf("initialized Sec %s Target %s\n", options.TargetKind, options.TargetName)
	}
	return nil
}

// planInitTarget inspects source and metadata conflicts, calculates precise
// additive patches and validates the proposed manifest before any mutation.
// Rules: rules/projects/projects.md — §§17–19,39–43.
func planInitTarget(options initCommandOptions) (*initPlan, string, error) {
	if !project.ValidProjectKey(options.TargetName) {
		return nil, "", fmt.Errorf("Target name must match [a-z][a-z0-9_-]*")
	}
	switch options.TargetKind {
	case "command", "library", "firmware", "test":
	default:
		return nil, "", fmt.Errorf("invalid Target kind %q", options.TargetKind)
	}
	path, found, err := discoverInitManifest(options.ProjectDir)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return nil, "", fmt.Errorf("sec init %s requires an existing project", options.TargetKind)
	}
	manifest, err := project.LoadManifest(path)
	if err != nil {
		return nil, "", err
	}
	doc, err := project.ParseManifestDocument(manifest.Text())
	if err != nil {
		return nil, "", err
	}
	table, err := doc.Table("target", options.TargetName)
	if err != nil {
		return nil, "", err
	}
	kind, err := initTargetString(table, "kind")
	if err != nil {
		return nil, "", err
	}
	if kind != "" && kind != options.TargetKind {
		return nil, "", fmt.Errorf("init conflict: target.%s.kind is %s, requested %s", options.TargetName, kind, options.TargetKind)
	}
	source, err := initTargetString(table, "source")
	if err != nil {
		return nil, "", err
	}
	if source == "" {
		source = initTargetSource(options.TargetKind, options.TargetName)
	}
	if err := validateInitTargetSource(source); err != nil {
		return nil, "", err
	}
	variants, err := initTargetVariants(table)
	if err != nil {
		return nil, "", err
	}
	variantName, variantFields, err := initTargetVariant(doc, options.Target, variants, options.TargetSet)
	if err != nil {
		return nil, "", err
	}
	if len(variants) == 0 {
		variants = []string{variantName}
	}
	variantValues := make([]string, len(variants))
	for i, name := range variants {
		variantValues[i] = project.ManifestString(name)
	}
	fields := []project.ManifestField{{Name: "kind", Value: project.ManifestString(options.TargetKind)}, {Name: "source", Value: project.ManifestString(source)}, {Name: "variants", Value: "[" + strings.Join(variantValues, ", ") + "]"}}
	// Existing artifact and optional settings belong to the user; absent artifact
	// already defaults to the Target name (§17(9)), so no replacement is needed.
	proposal, err := doc.AddMissingFields([]string{"target", options.TargetName}, fields)
	if err != nil {
		return nil, "", err
	}
	if len(variantFields) != 0 {
		updated, err := project.ParseManifestDocument(proposal)
		if err != nil {
			return nil, "", err
		}
		proposal, err = updated.AddMissingFields([]string{"variant", variantName}, variantFields)
		if err != nil {
			return nil, "", err
		}
	}
	resolved, failures := project.ParseManifest(path, proposal)
	if len(failures) != 0 {
		return nil, "", fmt.Errorf("proposed manifest is invalid: %v", failures[0])
	}
	root := manifest.Facts().ProjectRoot
	directory := filepath.Join(root, filepath.FromSlash(source))
	if directory != root {
		if err := rejectNestedInitManifest(directory); err != nil {
			return nil, "", err
		}
	}
	module, fileName, starter, err := initTargetStarter(resolved, options.TargetKind, source)
	if err != nil {
		return nil, "", err
	}
	if (options.TargetKind == "command" || options.TargetKind == "firmware") && source != "." {
		module = existingInitEntryModule(directory, module)
	}
	facts, err := inspectInitModuleSources(directory, module)
	if err != nil {
		return nil, "", err
	}
	if options.TargetKind == "command" || options.TargetKind == "firmware" {
		want := 0
		if facts.hasMainSource {
			want = 1
		}
		if facts.mainFunctions != want {
			return nil, "", fmt.Errorf("init source conflict: %s would not have exactly one main function", directory)
		}
		if want == 1 {
			if err := validateInitEntrySignature(facts, module, options.TargetKind); err != nil {
				return nil, "", err
			}
		}

	}
	plan := &initPlan{}
	if err := plan.addFile(filepath.Join(directory, fileName), []byte(starter), true); err != nil {
		return nil, "", err
	}
	if err := plan.addManifestEdit(path, []byte(manifest.Text()), []byte(proposal)); err != nil {
		return nil, "", err
	}
	// Snapshot unchanged metadata as well: a no-edit repeat must not ignore a
	// concurrent user edit between validation and applying source creates.
	if len(plan.edits) == 0 {
		if err := plan.addFile(path, []byte(manifest.Text()), false); err != nil {
			return nil, "", err
		}
	}
	return plan, path, nil
}

// initTargetString validates present scalar fields while leaving absent fields
// available for additive completion. Rules: rules/projects/projects.md — §§17,41–43.
func initTargetString(table map[string]any, key string) (string, error) {
	value, found := table[key]
	if !found {
		return "", nil
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("init conflict: Target %s must be a non-empty string", key)
	}
	return text, nil
}

// initTargetVariants requires the represented Target's non-empty variant list.
// Rules: rules/projects/projects.md — §§17(4),19,48.
func initTargetVariants(table map[string]any) ([]string, error) {
	value, found := table["variants"]
	if !found {
		return nil, nil
	}
	array, ok := value.([]any)
	if !ok || len(array) == 0 {
		return nil, fmt.Errorf("init conflict: Target variants must be a non-empty list of ProjectKeys")
	}
	names := make([]string, 0, len(array))
	for _, value := range array {
		name, ok := value.(string)
		if !ok || !project.ValidProjectKey(name) {
			return nil, fmt.Errorf("invalid Target variant name %v", value)
		}
		names = append(names, name)
	}
	return names, nil
}

// initTargetVariant preserves existing selections, or reuses a compatible
// registered platform Variant before requesting a new canonical declaration.
// Rules: rules/projects/projects.md — §§17(4),19,39(4); platform/target_profiles.md — target registry.
func initTargetVariant(doc *project.ManifestDocument, target CompilerTarget, names []string, explicit bool) (string, []project.ManifestField, error) {
	definition, ok := findTargetDefinition(target)
	if !ok {
		return "", nil, fmt.Errorf("unsupported target %s", target.String())
	}
	tables, err := doc.Table("variant")
	if err != nil {
		return "", nil, err
	}
	matching := func(name string) (bool, error) {
		if !project.ValidProjectKey(name) {
			return false, fmt.Errorf("invalid Variant ProjectKey %q", name)
		}
		table, err := doc.Table("variant", name)
		if err != nil {
			return false, err
		}
		if table == nil {
			return false, fmt.Errorf("Target references missing variant %s", name)
		}
		osName, okOS := table["os"].(string)
		arch, okArch := table["arch"].(string)
		if !okOS || !okArch {
			return false, fmt.Errorf("variant.%s requires os and arch strings", name)
		}
		variant, ok := parseCompilerTarget(osName + "-" + arch)
		if !ok {
			return false, fmt.Errorf("unsupported variant %s target %s-%s", name, osName, arch)
		}
		return variant == target, nil
	}
	if len(names) != 0 {
		match := false
		for _, name := range names {
			same, err := matching(name)
			if err != nil {
				return "", nil, err
			}
			match = match || same
		}
		if explicit && !match {
			return "", nil, fmt.Errorf("init conflict: existing Target does not select %s", target.String())
		}
		return names[0], nil, nil
	}
	candidates := make([]string, 0, len(tables))
	for name := range tables {
		candidates = append(candidates, name)
	}
	sort.Strings(candidates)
	for _, name := range candidates {
		same, err := matching(name)
		if err != nil {
			return "", nil, err
		}
		if same {
			return name, nil, nil
		}
	}
	name := target.String()
	if _, exists := tables[name]; exists {
		return "", nil, fmt.Errorf("init conflict: variant %s denotes a different target", name)
	}
	return name, []project.ManifestField{{Name: "os", Value: project.ManifestString(target.OS)}, {Name: "arch", Value: project.ManifestString(target.Arch)}, {Name: "llvm_triple", Value: project.ManifestString(definition.LLVMTriple)}}, nil
}
