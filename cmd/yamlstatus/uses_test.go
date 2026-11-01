package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func usesFixtures(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	writeFixture(t, dir, "index.yaml", `schema_version: 1
fragments:
  - path: core.yaml
  - path: library.yaml
    implementation_scope: library
  - path: docs.yml
`)
	writeFixture(t, dir, "core.yaml", `integrations:
  - id: core.done
    status: implemented
    code:
      - internal/shared.go
    tests: [internal/shared.go, internal/shared.go.bak]
    implemented: [mentions internal/shared.go]
  - id: core.prose
    status: planned
    summary: internal/shared.go
    remaining: [internal/shared.go]
`)
	writeFixture(t, dir, "library.yaml", `integrations:
  - id: library.one
    status: planned
    code: [./internal/shared.go]
`)
	writeFixture(t, dir, "docs.yml", `id: docs.one
status: partial
rules: ['internal\shared.go', {id: diagnostics.example, requirement: internal/shared.go}]
---
- id: docs.other
  status: partial
  code: [internal/other.go]
`)
	return root, dir
}

func usesCLI(t *testing.T, args ...string) PathUses {
	t.Helper()
	var out, errors bytes.Buffer
	if err := runCLI(args, &out, &errors); err != nil {
		t.Fatal(err, errors.String())
	}
	var report PathUses
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err, out.String())
	}
	return report
}

func TestPathUsesExactNormalizedLocationsAndReadOnly(t *testing.T) {
	root, dir := usesFixtures(t)
	originals := map[string][]byte{}
	for _, file := range []string{"core.yaml", "docs.yml", "library.yaml", "index.yaml"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		originals[file] = data
	}
	var first PathUses
	for _, query := range []string{"internal/shared.go", "./internal/sema/../shared.go", `internal\shared.go`, filepath.Join(root, "internal", "shared.go")} {
		report := usesCLI(t, "-dir", dir, "-uses", query, "-json")
		if report.Path != "internal/shared.go" || report.Integrations != 3 || len(report.References) != 4 {
			t.Fatal(report)
		}
		if first.Path == "" {
			first = report
		} else if !reflect.DeepEqual(first, report) {
			t.Fatal("normalization/order changed the report", first, report)
		}
		for _, reference := range report.References {
			data, err := os.ReadFile(reference.File)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(data), "\n")
			if reference.Line < 1 || reference.Line > len(lines) || !strings.Contains(lines[reference.Line-1], reference.Path) {
				t.Fatal("lost exact YAML scalar location", reference)
			}
			if reference.ID == "core.prose" || strings.HasSuffix(reference.Path, ".bak") || reference.Status == "" {
				t.Fatal("matched prose/prefix or lost completed/library owner", reference)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "shared.go")); !os.IsNotExist(err) {
		t.Fatal("fixture must prove missing paths remain queryable", err)
	}
	for file, original := range originals {
		after, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil || !bytes.Equal(after, original) {
			t.Fatal("query wrote a governance file", file, err)
		}
	}
}

func TestPathUsesFiltersEmptyAndHumanOutput(t *testing.T) {
	_, dir := usesFixtures(t)
	for _, filter := range [][]string{{"-id", "core.done"}, {"-file", "core.yaml"}, {"-dir", filepath.Join(dir, "core.yaml")}} {
		args := append([]string{"-dir", dir, "-uses", "internal/shared.go", "-json"}, filter...)
		report := usesCLI(t, args...)
		if report.Integrations != 1 || len(report.References) != 2 || report.References[0].ID != "core.done" {
			t.Fatal(report)
		}
	}
	report := usesCLI(t, "-dir", dir, "-uses", "missing.go", "-json")
	if report.Integrations != 0 || report.References == nil || len(report.References) != 0 {
		t.Fatal(report)
	}
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-uses", "internal/shared.go"}, &out, &errors); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 integrations", "4 listed paths", "core.yaml:5: core.done [implemented] code:", "docs.yml:3: docs.one [partial] rules:", "library.one [planned]"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestPathUsesRejectsIncompatibleOrAmbiguousRequests(t *testing.T) {
	_, dir := usesFixtures(t)
	for _, options := range [][]string{{"-random"}, {"-point", "1", "-id", "core.done"}, {"-exclude", "skip"}, {"-html"}, {"-itemonly"}, {"-verbose"}, {"-id", "unknown"}} {
		args := append([]string{"-dir", dir, "-uses", "internal/shared.go"}, options...)
		var out, errors bytes.Buffer
		if err := runCLI(args, &out, &errors); err == nil || out.Len() != 0 {
			t.Fatal("invalid request accepted or produced partial output", options, err, out.String())
		}
	}
	for _, path := range []string{"", "   "} {
		var out, errors bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-uses", path}, &out, &errors); err == nil {
			t.Fatal("empty query accepted")
		}
	}
	writeFixture(t, dir, "duplicate.yaml", "id: core.done\nstatus: partial\ncode: [internal/shared.go]\n")
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-uses", "internal/shared.go", "-id", "core.done"}, &out, &errors); err == nil || !strings.Contains(err.Error(), "multiple owners") {
		t.Fatal("ambiguous owner accepted", err)
	}
}

func TestPathUsesRejectsMalformedReferenceLists(t *testing.T) {
	for _, field := range []string{"code: internal/shared.go", "tests: [true]", "rules: [{path: internal/shared.go}]"} {
		dir := t.TempDir()
		writeFixture(t, dir, "bad.yaml", "id: bad\nstatus: partial\n"+field+"\n")
		var out, errors bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-uses", "internal/shared.go"}, &out, &errors); err == nil || !strings.Contains(err.Error(), "bad.yaml:3") || out.Len() != 0 {
			t.Fatal("malformed reference list was silently ignored", err, out.String())
		}
	}
}
