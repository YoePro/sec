package main

import "strings"

// productionDependencySources excludes test files before automatic dependency
// parsing, identity checks and analysis, preserving source order. Explicit test
// selections are handled separately by selectTestSourceGroups.
// Rules: rules/tooling/testing.md — §§4.3–4.4,25,28.4.
func productionDependencySources(paths []string) []string {
	selected := make([]string, 0, len(paths))
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.sec") {
			selected = append(selected, path)
		}
	}
	return selected
}
