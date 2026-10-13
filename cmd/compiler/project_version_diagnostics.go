package main

import (
	"os"
	"path/filepath"
	"sort"

	"sec/internal/sema"
)

// validateDiscoveredProjectVersions reports each governing manifest once,
// before body analysis. Source spans belong to sec.toml rather than the source
// file that caused discovery.
// Rules: rules/projects/projects.md — §§8–11,36,48(1–3).
func validateDiscoveredProjectVersions(sources []string) diagnosticSummary {
	paths := map[string]bool{}
	for _, source := range sources {
		if source != "" {
			start := filepath.Dir(source)
			if info, err := os.Stat(source); err == nil && info.IsDir() {
				start = source
			}
			if path, found := findNearestProjectManifest(start); found {
				paths[path] = true
			}
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	summary := diagnosticSummary{}
	for _, path := range ordered {
		text, err := os.ReadFile(path)
		if err != nil {
			reportToolError("project", "%v", err)
			summary.Errors++
			continue
		}
		for _, issue := range sema.ValidateProjectVersion(path, string(text)) {
			printSemaError(os.Stderr, issue)
			summary.Errors++
		}
	}
	return summary
}
