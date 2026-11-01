package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PathReference is a listed path, not a textual mention in an implementation
// note. Keep the scalar's exact YAML location so every affected list is editable.
type PathReference struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Field  string `json:"field"`
	Path   string `json:"path"`
}

type PathUses struct {
	Path         string          `json:"path"`
	Integrations int             `json:"integration_count"`
	References   []PathReference `json:"references"`
}

// referenceRoot anchors repository-relative paths at the ledger directory's
// parent, including when -dir points directly to a fragment. Queries do not
// require referenced files to exist and do not follow source-file symlinks.
// Contract: cmd/yamlstatus/IDEAS.md — §3; governance/README.md — canonical owners.
func referenceRoot(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	return filepath.Dir(absolute), nil
}

// normalizedReferencePath compares logical listed paths, preserving case and
// exact filenames. An absolute native path is relativized to the repository
// root; separators and redundant dot segments do not create false misses.
func normalizedReferencePath(path, root string) string {
	path = filepath.Clean(strings.ReplaceAll(path, "\\", "/"))
	if filepath.IsAbs(path) {
		if relative, err := filepath.Rel(root, path); err == nil {
			path = relative
		}
	}
	return filepath.ToSlash(path)
}

// findPathUses queries every integration, including completed and library
// entries. Only code/tests/rules lists count; prose and prefix matches do not.
// Contract: cmd/yamlstatus/IDEAS.md — §3 "-uses PATH".
func findPathUses(entries []Entry, query, root, id string) (PathUses, error) {
	report := PathUses{Path: normalizedReferencePath(query, root), References: []PathReference{}}
	owners := 0
	for _, entry := range entries {
		if id != "" && entry.Item.ID != id {
			continue
		}
		owners++
		used := false
		for _, field := range []string{"code", "tests", "rules"} {
			list := mappingValue(entry.Node, field)
			if list == nil || list.Tag == "!!null" {
				continue
			}
			if list.Kind != yaml.SequenceNode {
				return PathUses{}, fmt.Errorf("%s:%d: %s %s must be a path list", entry.File, list.Line, entry.Item.ID, field)
			}
			for _, path := range list.Content {
				// Some integrations list structured rule requirements with an
				// ID/paragraphs/requirement, rather than rulebook file paths.
				// Their prose is not a file reference.
				if field == "rules" && path.Kind == yaml.MappingNode {
					if ruleID := mappingValue(path, "id"); ruleID != nil && ruleID.Kind == yaml.ScalarNode && ruleID.Tag == "!!str" {
						continue
					}
				}
				if path.Kind != yaml.ScalarNode || path.Tag != "!!str" {
					return PathUses{}, fmt.Errorf("%s:%d: %s %s paths must be strings", entry.File, path.Line, entry.Item.ID, field)
				}
				if normalizedReferencePath(path.Value, root) == report.Path {
					report.References = append(report.References, PathReference{
						File: entry.File, Line: path.Line, ID: entry.Item.ID, Status: entry.Item.Status, Field: field, Path: path.Value,
					})
					used = true
				}
			}
		}
		if used {
			report.Integrations++
		}
	}
	if id != "" {
		if owners == 0 {
			return PathUses{}, fmt.Errorf("integration ID %q was not found", id)
		}
		if owners > 1 {
			return PathUses{}, fmt.Errorf("integration ID %q has multiple owners", id)
		}
	}
	sort.Slice(report.References, func(i, j int) bool {
		left, right := report.References[i], report.References[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Field != right.Field {
			return left.Field < right.Field
		}
		return left.Path < right.Path
	})
	return report, nil
}

// printPathUses keeps JSON free from a statistics banner and human output
// directly navigable. A valid query with no references is a successful report.
func printPathUses(out io.Writer, report PathUses, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	var text strings.Builder
	ownerLabel, verb, referenceLabel := "integrations", "reference", "listed paths"
	if report.Integrations == 1 {
		ownerLabel, verb = "integration", "references"
	}
	if len(report.References) == 1 {
		referenceLabel = "listed path"
	}
	fmt.Fprintf(&text, "%d %s %s %s (%d %s)\n", report.Integrations, ownerLabel, verb, report.Path, len(report.References), referenceLabel)
	for _, reference := range report.References {
		fmt.Fprintf(&text, "%s:%d: %s [%s] %s: %s\n", reference.File, reference.Line, reference.ID, reference.Status, reference.Field, reference.Path)
	}
	_, err := io.WriteString(out, text.String())
	return err
}
