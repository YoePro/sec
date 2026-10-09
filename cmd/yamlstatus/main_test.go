package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectGovernanceStats(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "index.yaml", "schema_version: 1\nfragments:\n  - path: core.yaml\n")
	writeFixture(t, dir, "core.yaml", `schema_version: 1
area: core
integrations:
  - id: core.one
    status: partial
    implemented: [first, second]
    remaining: [third]
    rules: [core.md]
  - id: core.two
    status: implemented
    implemented: [fourth]
    partial: [legacy]
    rules: [other.md]
`)
	writeFixture(t, dir, "nested/extra.YML", "integrations:\n  - id: extra.one\n    status: planned\n    remaining: [one, two]\n")
	writeFixture(t, dir, "README.md", "not YAML")
	stats, err := collectStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 3 || stats.TotalObjects != 3 || stats.TotalImplemented != 3 || stats.TotalRemaining != 3 || stats.TotalPartial != 1 {
		t.Fatalf("incorrect totals: %+v", stats)
	}
	for _, status := range []string{"partial", "implemented", "planned"} {
		if stats.StatusCounts[status] != 1 {
			t.Fatalf("incorrect status count: %+v", stats.StatusCounts)
		}
	}
	if len(stats.FieldCounts) != 1 || stats.FieldCounts["rules"] != 2 {
		t.Fatalf("metadata counted as integration fields: %+v", stats.FieldCounts)
	}
}

func TestCollectStatsReportsErrors(t *testing.T) {
	for _, content := range []string{"integrations: [", "integrations: invalid", "integrations:\n  - status: partial\n"} {
		t.Run(content, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, "broken.yaml", content)
			if _, err := collectStats(dir); err == nil || !strings.Contains(err.Error(), "broken.yaml") {
				t.Fatalf("expected error identifying file, got %v", err)
			}
		})
	}
	if _, err := collectStats(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing directory error")
	}
}

func TestStandaloneAndMultipleDocuments(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "entries.yaml", "id: one\nstatus: implemented\n---\n- id: two\n  remaining: [task]\n")
	stats, err := collectStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalObjects != 2 || stats.TotalRemaining != 1 || stats.StatusCounts["unspecified"] != 1 {
		t.Fatalf("incorrect totals: %+v", stats)
	}
}

func TestEnglishOutputAndSortedFields(t *testing.T) {
	var output bytes.Buffer
	printResults(&output, Stats{
		TotalObjects: 2,
		StatusCounts: map[string]int{"partial": 2},
		FieldCounts:  map[string]int{"zebra": 1, "alpha": 2},
	})
	text := output.String()
	for _, expected := range []string{"IMPLEMENTATION STATISTICS", "Total integrations:", "Integration statuses:", "Checklist items:", "Remaining:"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing English label %q", expected)
		}
	}
	if strings.Index(text, "alpha") >= strings.Index(text, "zebra") {
		t.Fatal("fields are not sorted")
	}
}
