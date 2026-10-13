package project

import (
	"fmt"
	"path/filepath"
)

// RootModuleName resolves the root source declaration independently of directory
// spelling. Root commands/firmware use main; a root library uses the override or
// the complete display name when it is a valid identifier.
// Rules: rules/projects/projects.md — §7(7–8),§14(1–9),§53(8).
func (manifest *Manifest) RootModuleName() (string, error) {
	entry, library := false, false
	for _, target := range manifest.targets {
		if filepath.Clean(filepath.FromSlash(target.Source)) != "." || target.Source == "" {
			continue
		}
		if target.Kind == "command" || target.Kind == "firmware" {
			entry = true
		} else {
			library = true
		}
	}
	name := manifest.identity.Module
	if name == "" {
		name = manifest.facts.ProjectName
	}
	if entry && !library {
		return "main", nil
	}
	if !ValidModuleName(name) {
		return "", fmt.Errorf("root library requires [project].module because project.name %q is not a valid ModuleName", manifest.facts.ProjectName)
	}
	if entry && name != "main" {
		return "", fmt.Errorf("root source cannot be both module main and root library module %s", name)
	}
	return name, nil
}
