package project

import (
	"path/filepath"
	"sort"
)

// Target is the part of a `[target.<name>]` table that source analysis needs:
// its kind (command, firmware, library, or test) and its source location.
//
// Rules:
//   - rules/projects/projects.md — § 17 "Targets"
type Target struct {
	Name   string
	Kind   string
	Source string
}

// ParseManifestTargets reads the kind and source of every `[target.<name>]`
// table of a manifest. Nested tables such as `[target.<name>.version]` and
// all other tables are skipped; malformed values are left empty so the
// caller falls back to its default.
func ParseManifestTargets(text string) []Target {

	targets := map[string]*Target{}
	doc, err := ParseManifestDocument(text)
	if err != nil {
		return nil
	}
	tables, err := doc.Table("target")
	if err != nil {
		return nil
	}
	for name, value := range tables {
		table, ok := value.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := table["kind"].(string)
		source, _ := table["source"].(string)
		targets[name] = &Target{Name: name, Kind: kind, Source: source}
	}

	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Target, 0, len(names))
	for _, name := range names {
		result = append(result, *targets[name])
	}
	return result
}

// TargetForSource returns the Target whose source is the given source file's
// module directory, or the file itself, under the project root.
//
// Rules:
//   - rules/projects/projects.md — § 17 "Targets", § 14 project-root source files
func TargetForSource(targets []Target, projectRoot string, sourcePath string) (Target, bool) {
	file := filepath.Clean(sourcePath)
	directory := filepath.Dir(file)
	for _, target := range targets {
		if target.Source == "" {
			continue
		}
		location := filepath.Clean(filepath.Join(projectRoot, filepath.FromSlash(target.Source)))
		if location == file || location == directory {
			return target, true
		}
	}
	return Target{}, false
}
