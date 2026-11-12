package main

import (
	"os"
	"path/filepath"

	"sec/internal/project"
	"sec/internal/sema"
)

// cliSourceSelection derives the source-selection facts of the concrete
// plan: the target identity selectors the plan defines (os and arch; cpu,
// device and board only when a target definition supplies them) and the
// typed compile-time parameters of the nearest project manifest, with the
// owning Target's overrides. Parameter problems are returned for reporting.
//
// Rules:
//   - rules/foundations/attributes.md — §§ 13, 17, 27
//   - rules/projects/projects.md — § 24
func cliSourceSelection(target CompilerTarget, sourceFiles []string) (sema.SourceSelection, []error) {
	selection := sema.SourceSelection{
		Target:     map[string]string{"os": normalizeTargetOS(target.OS), "arch": normalizeTargetArch(target.Arch)},
		Parameters: map[string]sema.CompileTimeParameter{},
		Normalize:  normalizeSelectorValue,
	}
	if len(sourceFiles) == 0 {
		return selection, nil
	}
	manifest, found := findNearestProjectManifest(filepath.Dir(sourceFiles[0]))
	if !found {
		return selection, nil
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return selection, nil
	}
	targetName := ""
	absolute, err := filepath.Abs(sourceFiles[0])
	if err != nil {
		absolute = sourceFiles[0]
	}
	root := filepath.Dir(filepath.Dir(manifest))
	targets := project.ParseManifestTargets(string(data))
	if resolved, diagnostics := project.ParseManifest(manifest, string(data)); len(diagnostics) == 0 {
		root = resolved.Facts().ProjectRoot
		targets = resolved.Targets()
	}
	if owner, ok := project.TargetForSource(targets, root, absolute); ok {
		targetName = owner.Name
	}
	parameters, problems := project.ParseManifestParameters(string(data), targetName)
	for name, parameter := range parameters {
		selection.Parameters[name] = sema.CompileTimeParameter{Kind: parameter.Kind, Bool: parameter.Bool, Text: parameter.Text}
	}
	return selection, problems
}

// normalizeSelectorValue canonicalizes os and arch selector spellings the
// way target names are normalized; other selectors compare exactly
// (attributes.md § 20(3)).
func normalizeSelectorValue(selector, value string) string {
	switch selector {
	case "os":
		return normalizeTargetOS(value)
	case "arch":
		return normalizeTargetArch(value)
	}
	return value
}
