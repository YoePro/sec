package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Item is an integration entry, whose checklist items can be strings or mappings.
type Item struct {
	ID          string               `yaml:"id"`
	Status      string               `yaml:"status"`
	Implemented []interface{}        `yaml:"implemented"`
	Remaining   []interface{}        `yaml:"remaining"`
	Partial     []interface{}        `yaml:"partial"`
	Extra       map[string]yaml.Node `yaml:",inline"`
}

// Stats keeps integration statuses separate from checklist item counts.
type Stats struct {
	Entries          []Entry
	Files            int
	TotalObjects     int
	TotalImplemented int
	TotalRemaining   int
	TotalPartial     int
	StatusCounts     map[string]int
	FieldCounts      map[string]int
}

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func collectStats(dir string) (Stats, error) {
	stats := Stats{StatusCounts: make(map[string]int), FieldCounts: make(map[string]int)}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		if err := processYAMLFile(path, &stats); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		stats.Files++
		return nil
	})
	return stats, err
}

// processYAMLFile reads wrapped governance entries, standalone entries and lists.
// Registry metadata (such as index.yaml) does not count as an integration.
func processYAMLFile(path string, stats *Stats) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	for {
		var root yaml.Node
		if err := decoder.Decode(&root); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(root.Content) == 0 {
			continue
		}
		node := root.Content[0]
		var items []Item
		var itemNodes []*yaml.Node
		switch node.Kind {
		case yaml.SequenceNode:
			itemNodes = node.Content
			if err := node.Decode(&items); err != nil {
				return err
			}
		case yaml.MappingNode:
			var document struct {
				ID           string    `yaml:"id"`
				Integrations yaml.Node `yaml:"integrations"`
			}
			if err := node.Decode(&document); err != nil {
				return err
			}
			if document.Integrations.Kind != 0 {
				if document.Integrations.Kind != yaml.SequenceNode {
					return fmt.Errorf("integrations must be a list")
				}
				itemNodes = document.Integrations.Content
				if err := document.Integrations.Decode(&items); err != nil {
					return err
				}
			} else if document.ID != "" {
				var item Item
				if err := node.Decode(&item); err != nil {
					return err
				}
				items = append(items, item)
				itemNodes = append(itemNodes, node)
			}
		default:
			if node.Tag != "!!null" {
				return fmt.Errorf("expected a YAML mapping or list")
			}
		}
		for index, item := range items {
			if item.ID == "" {
				return fmt.Errorf("integration is missing its id")
			}
			stats.Entries = append(stats.Entries, Entry{Item: item, File: path, Node: itemNodes[index]})
			stats.TotalObjects++
			stats.TotalImplemented += len(item.Implemented)
			stats.TotalRemaining += len(item.Remaining)
			stats.TotalPartial += len(item.Partial)
			status := item.Status
			if status == "" {
				status = "unspecified"
			}
			stats.StatusCounts[status]++
			for field := range item.Extra {
				stats.FieldCounts[field]++
			}
		}
	}
}

func printResults(out io.Writer, stats Stats) {
	fmt.Fprintln(out, "\nIMPLEMENTATION STATISTICS")
	fmt.Fprintf(out, "YAML files read:             %d\n", stats.Files)
	fmt.Fprintf(out, "Total integrations:          %d\n", stats.TotalObjects)
	fmt.Fprintln(out, "\nIntegration statuses:")
	for _, status := range []string{"implemented", "partial", "planned"} {
		fmt.Fprintf(out, "  %-20s %d\n", status, stats.StatusCounts[status])
	}
	for _, status := range sortedKeys(stats.StatusCounts) {
		if status != "implemented" && status != "partial" && status != "planned" {
			fmt.Fprintf(out, "  %-20s %d\n", status, stats.StatusCounts[status])
		}
	}
	fmt.Fprintln(out, "\nChecklist items:")
	fmt.Fprintf(out, "  Implemented:               %d\n", stats.TotalImplemented)
	fmt.Fprintf(out, "  Remaining:                 %d\n", stats.TotalRemaining)
	fmt.Fprintf(out, "  Partial:                   %d\n", stats.TotalPartial)
	fmt.Fprintln(out, "\nOther field occurrences (per integration):")
	for _, field := range sortedKeys(stats.FieldCounts) {
		fmt.Fprintf(out, "  %-25s %d\n", field, stats.FieldCounts[field])
	}
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
