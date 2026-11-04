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

// MissingPaths is a read-only existence report; each repeated listing retains
// its own editable YAML location, including evidence and nested plan lists.
type MissingPaths struct {
	Checked int             `json:"checked_references"`
	Missing []PathReference `json:"missing"`
}

// listedPathReferences walks code/tests/rules lists without treating prose or
// structured rule requirements as filenames. Other maps, including evidence
// and nested implementation plans, are visited in source order.
// Contract: cmd/yamlstatus/IDEAS.md — §2; governance/README.md — fragment contract.
func listedPathReferences(entry Entry) ([]PathReference, error) {
	var refs []PathReference
	var walk func(*yaml.Node, string) error
	walk = func(node *yaml.Node, prefix string) error {
		switch node.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(node.Content); i += 2 {
				key, value := node.Content[i], node.Content[i+1]
				field := key.Value
				if prefix != "" {
					field = prefix + "." + field
				}
				if key.Value != "code" && key.Value != "tests" && key.Value != "rules" {
					if err := walk(value, field); err != nil {
						return err
					}
					continue
				}
				if value.Tag == "!!null" {
					continue
				}
				if value.Kind != yaml.SequenceNode {
					return fmt.Errorf("%s:%d: %s %s must be a path list", entry.File, value.Line, entry.Item.ID, field)
				}
				for _, path := range value.Content {
					if key.Value == "rules" && path.Kind == yaml.MappingNode {
						if id := mappingValue(path, "id"); id != nil && id.Kind == yaml.ScalarNode && id.Tag == "!!str" {
							continue
						}
					}
					if path.Kind != yaml.ScalarNode || path.Tag != "!!str" || strings.TrimSpace(path.Value) == "" {
						return fmt.Errorf("%s:%d: %s %s paths must be non-empty strings", entry.File, path.Line, entry.Item.ID, field)
					}
					// Nested rule plans can list paragraph identifiers rather than
					// files (for example formatter obligations use "§ 4(1)").
					if key.Value == "rules" && strings.HasPrefix(strings.TrimSpace(path.Value), "§") {
						continue
					}
					refs = append(refs, PathReference{File: entry.File, Line: path.Line, ID: entry.Item.ID,
						Status: entry.Item.Status, Field: field, Path: path.Value})
				}
			}
		case yaml.SequenceNode:
			for i, child := range node.Content {
				if err := walk(child, fmt.Sprintf("%s[%d]", prefix, i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := walk(entry.Node, "")
	return refs, err
}

// checkReferencedPaths resolves logical paths from the ledger's repository
// root, including absolute native paths and normalized separators. Broken links
// are missing; permission/I/O failures are errors, never false missing reports.
// Contract: cmd/yamlstatus/IDEAS.md — §2, optional -go filter.
func checkReferencedPaths(entries []Entry, root, id string, goOnly bool) (MissingPaths, error) {
	report := MissingPaths{Missing: []PathReference{}}
	owners := 0
	for _, entry := range entries {
		if id != "" && entry.Item.ID != id {
			continue
		}
		owners++
		refs, err := listedPathReferences(entry)
		if err != nil {
			return MissingPaths{}, err
		}
		for _, ref := range refs {
			path := normalizedReferencePath(ref.Path, root)
			if goOnly && (filepath.Ext(path) != ".go" || !(strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/"))) {
				continue
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, filepath.FromSlash(path))
			}
			report.Checked++
			if _, err := os.Stat(path); err != nil {
				if !os.IsNotExist(err) {
					return MissingPaths{}, fmt.Errorf("%s:%d: %s %s %q: %w", ref.File, ref.Line, ref.ID, ref.Field, ref.Path, err)
				}
				report.Missing = append(report.Missing, ref)
			}
		}
	}
	if id != "" && owners != 1 {
		return MissingPaths{}, fmt.Errorf("integration ID %q must have exactly one owner (found %d)", id, owners)
	}
	sort.SliceStable(report.Missing, func(i, j int) bool {
		if report.Missing[i].File != report.Missing[j].File {
			return report.Missing[i].File < report.Missing[j].File
		}
		return report.Missing[i].Line < report.Missing[j].Line
	})
	return report, nil
}

// printMissingPaths emits a complete report even when findings cause the CLI
// to exit nonzero. JSON stdout contains only the report, with an empty array
// for a successful check; human findings use navigable source locations.
// Contract: cmd/yamlstatus/IDEAS.md — §2.
func printMissingPaths(out io.Writer, report MissingPaths, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	for _, ref := range report.Missing {
		if _, err := fmt.Fprintf(out, "%s:%d: %s [%s] %s: %s (missing)\n", ref.File, ref.Line, ref.ID, ref.Status, ref.Field, ref.Path); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "%d missing path references; %d checked\n", len(report.Missing), report.Checked)
	return err
}
