package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func selectionFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "core.yaml", `integrations:
  - id: core.one
    status: partial
    rules: [rules/core.md]
    remaining:
      - bootstrap the compiler
      - implement helper visibility
  - id: core.done
    status: implemented
    remaining: []
`)
	writeFixture(t, dir, "types.yaml", `integrations:
  - id: types.one
    status: planned
    remaining:
      - task: validate type
        evidence: missing
`)
	return dir
}

func TestSelectionFiltersAndProvenance(t *testing.T) {
	dir := selectionFixtures(t)
	for _, file := range []string{"core.yaml", filepath.Join(dir, "core.yaml")} {
		var out, errOut bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-file", file, "-id", "core.one", "-point", "2", "-json"}, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		var point SelectedPoint
		if err := json.Unmarshal(out.Bytes(), &point); err != nil {
			t.Fatal(err)
		}
		if point.ID != "core.one" || point.PointIndex != 2 || point.Remaining != 2 || point.Point != "implement helper visibility" || point.Line != 7 || point.File != filepath.Join(dir, "core.yaml") {
			t.Fatalf("wrong point/provenance: %+v", point)
		}
		if point.Integration["rules"] == nil {
			t.Fatal("missing rulebook context")
		}
	}
}

func TestRandomSelectionAndExclusion(t *testing.T) {
	dir := selectionFixtures(t)
	stats, err := collectStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		point, err := selectPoint(stats.Entries, "core.one", 0, "BOOTSTRAP", true)
		if err != nil || point.PointIndex != 2 {
			t.Fatalf("random selection escaped filter: %+v, %v", point, err)
		}
	}
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-file", "types.yaml", "-random", "-json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var point SelectedPoint
	if err := json.Unmarshal(out.Bytes(), &point); err != nil {
		t.Fatal(err)
	}
	if point.ID != "types.one" || point.Point.(map[string]interface{})["task"] != "validate type" {
		t.Fatalf("structured point was lost: %+v", point)
	}
}

func TestSelectionErrors(t *testing.T) {
	dir := selectionFixtures(t)
	for i, args := range [][]string{
		{"-id", "missing"},
		{"-id", "core.done"},
		{"-file", "types.yaml", "-id", "core.one"},
		{"-id", "core.one", "-point", "3"},
		{"-id", "core.one", "-exclude", "", "-point", "-1"},
		{"-point", "1"},
		{"-id", "core.one", "-point", "1", "-random"},
		{"-json"},
		{"-file", "missing.yaml"},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := runCLI(append([]string{"-dir", dir}, args...), &out, &errOut); err == nil {
				t.Fatalf("expected failure for %v", args)
			}
			if out.Len() != 0 {
				t.Fatal("printed a result for a failed selection")
			}
		})
	}
	writeFixture(t, dir, "duplicate.yaml", "integrations:\n  - id: core.one\n    remaining: [duplicate]\n")
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-id", "core.one"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "multiple owners") {
		t.Fatalf("expected duplicate ID error, got %v", err)
	}
}

func TestStatsAndHelpRemainAvailable(t *testing.T) {
	dir := selectionFixtures(t)
	var out, errOut bytes.Buffer
	if err := runCLI([]string{dir}, &out, &errOut); err != nil || !strings.Contains(out.String(), "IMPLEMENTATION STATISTICS") {
		t.Fatalf("statistics failed: %v, %s", err, &out)
	}
	if err := runCLI([]string{"-help"}, &out, &errOut); err != nil || !strings.Contains(errOut.String(), "-random") {
		t.Fatalf("help failed: %v, %s", err, &errOut)
	}
}
