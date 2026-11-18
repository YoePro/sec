package main

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

// fileChecklistCounts retains canonical checklist counts for one parsed YAML
// input, including metadata-only files and library files excluded from compiler
// totals. These are checklist entries, not integration-status counts.
type fileChecklistCounts struct {
	File                            string
	Implemented, Remaining, Partial int
	Library                         bool
}

// printFileChecklists presents each file in path order with percentages of its
// own three checklist counts. It never merges library counts into compiler
// totals or treats empty input as completed work.
func printFileChecklists(out io.Writer, files []fileChecklistCounts) error {
	if len(files) == 0 {
		_, err := fmt.Fprintln(out, "No YAML files found.")
		return err
	}
	ordered := append([]fileChecklistCounts(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].File < ordered[j].File })
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)

	written, err := fmt.Fprintln(table, "File\tImplemented\t Remaining\t  Partial")
	if err != nil {
		return err
	}
	if written > 0 {
		if _, err := fmt.Fprintln(table, "----\t-----------\t-----------\t----------"); err != nil {
			return err
		}
	}
	hasLibrary := false
	for _, file := range ordered {
		name := file.File
		if file.Library {
			name += " [library]"
			hasLibrary = true
		}
		total := file.Implemented + file.Remaining + file.Partial
		percent := func(count int) float64 {
			if total == 0 {
				return 0
			}
			return float64(count) / float64(total) * 100
		}
		if _, err := fmt.Fprintf(table, "%s\t%3d (%.1f%%)\t%3d (%.1f%%)\t%3d (%.1f%%)\n",
			name, file.Implemented, percent(file.Implemented),
			file.Remaining, percent(file.Remaining), file.Partial, percent(file.Partial)); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if hasLibrary {
		_, err := fmt.Fprintln(out, "\n[library] Files are excluded from compiler implementation totals.")
		return err
	}
	return nil
}
