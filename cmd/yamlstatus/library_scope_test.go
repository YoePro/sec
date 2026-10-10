package main

import (
	"bytes"
	"strings"
	"testing"
)

// Fragments the index marks implementation_scope: library (the standard
// library) are not compiler implementation: their integrations stay out of
// every statistic, are reported separately, and remain selectable.
func TestLibraryFragmentsAreNotCountedAsCompilerImplementation(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "index.yaml", `schema_version: 1
fragments:
  - path: core.yaml
    owns: core
  - path: stdlib.yaml
    owns: library
    implementation_scope: library
`)
	writeFixture(t, dir, "core.yaml", "integrations:\n  - id: core.one\n    status: partial\n    implemented: [a]\n    remaining: [b]\n")
	writeFixture(t, dir, "stdlib.yaml", "integrations:\n  - id: stdlib.one\n    status: planned\n    remaining: [c, d, e]\n")
	stats, err := collectStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalObjects != 1 || stats.TotalImplemented != 1 || stats.TotalRemaining != 1 || stats.StatusCounts["planned"] != 0 {
		t.Fatalf("library integration counted: %+v", stats)
	}
	if stats.LibraryObjects != 1 || len(stats.LibraryFiles) != 1 || stats.LibraryFiles[0] != "stdlib.yaml" {
		t.Fatalf("library integration not reported: %+v", stats)
	}
	if len(stats.Entries) != 2 {
		t.Fatalf("library integration not selectable: %d entries", len(stats.Entries))
	}
	var out bytes.Buffer
	printResults(&out, stats, false)
	if !strings.Contains(out.String(), "Not counted (library, not compiler implementation): 1 integrations in stdlib.yaml") {
		t.Fatalf("output does not report the library fragment:\n%s", out.String())
	}
	selected, err := selectPoint(stats.Entries, "stdlib.one", 0, "", false)
	if err != nil || selected.ID != "stdlib.one" {
		t.Fatalf("library point not selectable: %+v %v", selected, err)
	}
}
