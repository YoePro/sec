package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type RemainingMatch struct {
	File       string      `json:"file"`
	Line       int         `json:"line"`
	ID         string      `json:"id"`
	Status     string      `json:"status"`
	PointIndex int         `json:"point_index"`
	Remaining  int         `json:"remaining_count"`
	Point      interface{} `json:"point"`
}

type RemainingSearch struct {
	Query   string           `json:"query"`
	Matches []RemainingMatch `json:"matches"`
}

// remainingTextMatches searches decoded item text, not YAML serialization or
// mapping keys. Literal case-insensitive matching includes structured item IDs
// and descriptions/expectations, without turning punctuation into a regex.
// Contract: cmd/yamlstatus/IDEAS.md — §9.
func remainingTextMatches(node *yaml.Node, query string) bool {
	if node.Kind == yaml.ScalarNode {
		return node.Tag == "!!str" && strings.Contains(strings.ToLower(node.Value), query)
	}
	for i, child := range node.Content {
		if node.Kind == yaml.MappingNode && i%2 == 0 {
			continue
		}
		if remainingTextMatches(child, query) {
			return true
		}
	}
	return false
}

// searchRemaining lists matching top-level remaining checklist items, exactly
// the work counted/selectable by yamlstatus. Completed prose, partial lists,
// metadata, and nested implementation-plan notes are not candidates. Library
// integrations remain searchable, even though compiler statistics exclude them.
// Contract: cmd/yamlstatus/IDEAS.md — §9; codex.md — selection/counting contract.
func searchRemaining(entries []Entry, query, id string) (RemainingSearch, error) {
	report := RemainingSearch{Query: query, Matches: []RemainingMatch{}}
	needle := strings.ToLower(query)
	owners := 0
	for _, entry := range entries {
		if id != "" && entry.Item.ID != id {
			continue
		}
		owners++
		for i, point := range entry.Item.Remaining {
			node := remainingPointNode(entry, i)
			if remainingTextMatches(node, needle) {
				report.Matches = append(report.Matches, RemainingMatch{File: entry.File,
					Line: node.Line, ID: entry.Item.ID, Status: entry.Item.Status,
					PointIndex: i + 1, Remaining: len(entry.Item.Remaining), Point: point})
			}
		}
	}
	if id != "" && owners != 1 {
		return RemainingSearch{}, fmt.Errorf("integration ID %q must have exactly one owner (found %d)", id, owners)
	}
	sort.SliceStable(report.Matches, func(i, j int) bool {
		left, right := report.Matches[i], report.Matches[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.PointIndex < right.PointIndex
	})
	return report, nil
}

// printRemainingSearch emits navigable matches or one JSON report. A valid
// query with zero matches succeeds, including a selected owner with no work.
// Contract: cmd/yamlstatus/IDEAS.md — §9.
func printRemainingSearch(out io.Writer, report RemainingSearch, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	if _, err := fmt.Fprintf(out, "%d remaining points match %q\n", len(report.Matches), report.Query); err != nil {
		return err
	}
	for _, match := range report.Matches {
		encoded, err := yaml.Marshal(match.Point)
		if err != nil {
			return err
		}
		indented := "  " + strings.ReplaceAll(strings.TrimSuffix(string(encoded), "\n"), "\n", "\n  ")
		if _, err := fmt.Fprintf(out, "%s:%d: %s [%s] remaining[%d/%d]:\n%s\n",
			match.File, match.Line, match.ID, match.Status, match.PointIndex, match.Remaining, indented); err != nil {
			return err
		}
	}
	return nil
}
