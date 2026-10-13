package main

import (
	"os"
	"path/filepath"

	"sec/internal/project"
)

// validateBuildCounterManifest enforces counter declarations in discovered build
// projects through the shared model. General discovered-manifest diagnostics
// remain separate work; projects without any counter retain their legacy path.
// This validation does not reserve, advance or commit an automatic candidate.
// Rules: rules/projects/projects.md — §§28,48(1–2),49(1–2).
func validateBuildCounterManifest(sourceFile string) error {
	path, found := findNearestProjectManifest(filepath.Dir(sourceFile))
	if !found {
		return nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := project.ParseManifestDocument(string(text))
	if err != nil {
		return err
	}
	targets, err := doc.Table("target")
	if err != nil {
		return err
	}
	for _, value := range targets {
		if table, ok := value.(map[string]any); ok {
			if _, declared := table["build_counter"]; declared {
				_, err := project.ParseManifest(path, string(text))
				if len(err) != 0 {
					return &project.ManifestErrors{Path: path, Diagnostics: err}
				}
				return nil
			}
		}
	}
	return nil
}
