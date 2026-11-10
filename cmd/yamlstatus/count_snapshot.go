package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type checklistCounts struct {
	Remaining   int `json:"remaining"`
	Implemented int `json:"implemented"`
	Partial     int `json:"partial"`
}
type countEntry struct {
	ID     string          `json:"id"`
	File   string          `json:"file"`
	Scope  string          `json:"scope"`
	Status string          `json:"status"`
	Counts checklistCounts `json:"counts"`
}
type countSnapshot struct {
	SchemaVersion int          `json:"schema_version"`
	Entries       []countEntry `json:"entries"`
}

// snapshotCounts captures canonical checklist counts, keeping library owners
// separate and using only ledger-relative paths in shareable snapshots.
// Contract: cmd/yamlstatus/IDEAS.md — §7; codex.md — Governance, Private paths.
func snapshotCounts(dir string, stats Stats) (countSnapshot, error) {
	snapshot := countSnapshot{SchemaVersion: 1, Entries: []countEntry{}}
	library := map[string]bool{}
	for _, file := range stats.LibraryFiles {
		library[file] = true
	}
	for _, entry := range stats.Entries {
		file := relativeFragment(dir, entry.File)
		scope := "compiler"
		if library[file] {
			scope = "library"
		}
		snapshot.Entries = append(snapshot.Entries, countEntry{entry.Item.ID, file, scope, entry.Item.Status, checklistCounts{len(entry.Item.Remaining), len(entry.Item.Implemented), len(entry.Item.Partial)}})
	}
	sort.Slice(snapshot.Entries, func(i, j int) bool { return snapshot.Entries[i].ID < snapshot.Entries[j].ID })
	return snapshot, validateCountSnapshot(snapshot)
}

// validateCountSnapshot rejects incompatible versions, duplicate identities,
// malformed counts and nonportable paths before comparison.
// Contract: cmd/yamlstatus/IDEAS.md — §7; codex.md — Private paths.
func validateCountSnapshot(snapshot countSnapshot) error {
	if snapshot.SchemaVersion != 1 {
		return fmt.Errorf("unsupported count snapshot schema version %d", snapshot.SchemaVersion)
	}
	ids := map[string]bool{}
	for _, entry := range snapshot.Entries {
		if strings.TrimSpace(entry.ID) == "" || ids[entry.ID] {
			return fmt.Errorf("empty or duplicate integration ID %q in count snapshot", entry.ID)
		}
		ids[entry.ID] = true
		if entry.File == "" || path.IsAbs(entry.File) || path.Clean(entry.File) != entry.File || entry.File == ".." || strings.HasPrefix(entry.File, "../") || strings.ContainsAny(entry.File, "\\:") {
			return fmt.Errorf("snapshot requires ledger-relative fragment paths")
		}
		if entry.Scope != "compiler" && entry.Scope != "library" {
			return fmt.Errorf("unknown implementation scope %q", entry.Scope)
		}
		if entry.Counts.Remaining < 0 || entry.Counts.Implemented < 0 || entry.Counts.Partial < 0 {
			return fmt.Errorf("negative checklist count for %s", entry.ID)
		}
		if entry.Status != "implemented" && entry.Status != "partial" && entry.Status != "planned" {
			return fmt.Errorf("unknown integration status %q", entry.Status)
		}
	}
	return nil
}

// writeCountSnapshot creates a new baseline without overwriting an existing
// file or modifying the ledger. Encode before opening so encoding cannot leave
// a partial baseline behind.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func writeCountSnapshot(file string, snapshot countSnapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(file)
	}
	return err
}

// readCountSnapshot accepts exactly one versioned snapshot document and fails
// closed on unknown fields or trailing data.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func readCountSnapshot(file string) (countSnapshot, error) {
	f, err := os.Open(file)
	if err != nil {
		return countSnapshot{}, err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var snapshot countSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return snapshot, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return snapshot, fmt.Errorf("count snapshot must contain exactly one JSON document")
	}
	return snapshot, validateCountSnapshot(snapshot)
}
