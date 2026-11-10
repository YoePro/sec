package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type countGroupChange struct {
	Scope  string          `json:"scope"`
	Name   string          `json:"name"`
	Before checklistCounts `json:"before"`
	After  checklistCounts `json:"after"`
	Delta  checklistCounts `json:"delta"`
}
type integrationCountChange struct {
	ID     string          `json:"id"`
	Before *countEntry     `json:"before"`
	After  *countEntry     `json:"after"`
	Delta  checklistCounts `json:"delta"`
}
type countDiff struct {
	SchemaVersion int                      `json:"schema_version"`
	Totals        []countGroupChange       `json:"totals"`
	Fragments     []countGroupChange       `json:"fragments"`
	Integrations  []integrationCountChange `json:"integrations"`
}

// addCounts adds exact checklist totals without reclassifying partial work.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func addCounts(a, b checklistCounts) checklistCounts {
	return checklistCounts{a.Remaining + b.Remaining, a.Implemented + b.Implemented, a.Partial + b.Partial}
}

// subtractCounts reports after-minus-before, so remaining work removed is negative.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func subtractCounts(a, b checklistCounts) checklistCounts {
	return checklistCounts{a.Remaining - b.Remaining, a.Implemented - b.Implemented, a.Partial - b.Partial}
}

// compareCounts matches stable IDs, retaining additions/removals, file moves,
// scope changes and status-only changes. Output order never depends on maps.
// Contract: cmd/yamlstatus/IDEAS.md — §7; codex.md — Governance.
func compareCounts(before, after countSnapshot) countDiff {
	report := countDiff{SchemaVersion: 1, Totals: []countGroupChange{}, Fragments: []countGroupChange{}, Integrations: []integrationCountChange{}}
	old, new := map[string]countEntry{}, map[string]countEntry{}
	groups := map[[2]string]*countGroupChange{}
	totals := map[string]*countGroupChange{}
	for side, snapshot := range []countSnapshot{before, after} {
		for _, entry := range snapshot.Entries {
			if side == 0 {
				old[entry.ID] = entry
			} else {
				new[entry.ID] = entry
			}
			key := [2]string{entry.Scope, entry.File}
			if groups[key] == nil {
				groups[key] = &countGroupChange{Scope: entry.Scope, Name: entry.File}
			}
			if totals[entry.Scope] == nil {
				totals[entry.Scope] = &countGroupChange{Scope: entry.Scope, Name: entry.Scope}
			}
			for _, group := range []*countGroupChange{groups[key], totals[entry.Scope]} {
				if side == 0 {
					group.Before = addCounts(group.Before, entry.Counts)
				} else {
					group.After = addCounts(group.After, entry.Counts)
				}
			}
		}
	}
	ids := map[string]bool{}
	for id := range old {
		ids[id] = true
	}
	for id := range new {
		ids[id] = true
	}
	for id := range ids {
		b, bok := old[id]
		a, aok := new[id]
		if bok && aok && b == a {
			continue
		}
		change := integrationCountChange{ID: id, Delta: subtractCounts(a.Counts, b.Counts)}
		if bok {
			change.Before = &b
		}
		if aok {
			change.After = &a
		}
		report.Integrations = append(report.Integrations, change)
	}
	for _, group := range groups {
		group.Delta = subtractCounts(group.After, group.Before)
		if group.Before != group.After {
			report.Fragments = append(report.Fragments, *group)
		}
	}
	for _, group := range totals {
		group.Delta = subtractCounts(group.After, group.Before)
		report.Totals = append(report.Totals, *group)
	}
	sort.Slice(report.Integrations, func(i, j int) bool { return report.Integrations[i].ID < report.Integrations[j].ID })
	less := func(a, b countGroupChange) bool {
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.Name < b.Name
	}
	sort.Slice(report.Fragments, func(i, j int) bool { return less(report.Fragments[i], report.Fragments[j]) })
	sort.Slice(report.Totals, func(i, j int) bool { return less(report.Totals[i], report.Totals[j]) })
	return report
}

// printCountDiff presents compiler/library totals and changed fragments/IDs;
// a count reduction is not claimed as implementation if an owner was removed.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func printCountDiff(out io.Writer, report countDiff, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	printCounts := func(name string, before, after, delta checklistCounts) {
		fmt.Fprintf(out, "%s: remaining %d -> %d (%+d), implemented %d -> %d (%+d), partial %d -> %d (%+d)\n", name, before.Remaining, after.Remaining, delta.Remaining, before.Implemented, after.Implemented, delta.Implemented, before.Partial, after.Partial, delta.Partial)
	}
	for _, group := range report.Totals {
		printCounts(group.Scope, group.Before, group.After, group.Delta)
	}
	fmt.Fprintln(out, "Changed fragments:")
	for _, group := range report.Fragments {
		printCounts(group.Scope+"/"+group.Name, group.Before, group.After, group.Delta)
	}
	fmt.Fprintln(out, "Changed integrations:")
	for _, change := range report.Integrations {
		var b, a checklistCounts
		label := change.ID
		if change.Before != nil {
			b = change.Before.Counts
		}
		if change.After != nil {
			a = change.After.Counts
		}
		switch {
		case change.Before == nil:
			label += " [added]"
		case change.After == nil:
			label += " [removed]"
		default:
			if change.Before.File != change.After.File {
				label += " [moved: " + change.Before.File + " -> " + change.After.File + "]"
			}
			if change.Before.Scope != change.After.Scope {
				label += " [scope: " + change.Before.Scope + " -> " + change.After.Scope + "]"
			}
			if change.Before.Status != change.After.Status {
				label += " [status: " + change.Before.Status + " -> " + change.After.Status + "]"
			}
		}
		printCounts(label, b, a, change.Delta)
	}
	fmt.Fprintf(out, "%d changed integrations\n", len(report.Integrations))
	return nil
}
