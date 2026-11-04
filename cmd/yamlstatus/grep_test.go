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

func remainingSearchFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "index.yaml", "fragments:\n  - path: work.yaml\n  - path: library.yaml\n    implementation_scope: library\n")
	writeFixture(t, dir, "work.yaml", `integrations:
  - id: work.one
    status: partial
    summary: hidden only
    implemented: [hidden only]
    partial: [hidden only]
    remaining:
      - Bind C::Value with [ref]
      - id: MD-021
        description: >-
          Resolve the ÅÄÖ rule
          across line two
      - id: own.test
        expectation: 'Keep C:: signatures'
    plans:
      - remaining: [hidden only]
  - id: work.done
    status: implemented
    implemented: [C:: historical]
---
id: work.more
status: planned
remaining: [other work, 'C:: additional']
`)
	writeFixture(t, dir, "library.yaml", "id: library.one\nstatus: planned\nremaining: ['C:: library']\n")
	return dir
}

func searchCLI(t *testing.T, args ...string) RemainingSearch {
	t.Helper()
	var out, errors bytes.Buffer
	if err := runCLI(args, &out, &errors); err != nil {
		t.Fatal(err, errors.String())
	}
	var report RemainingSearch
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err, out.String())
	}
	return report
}

func TestRemainingSearchLiteralTextAndLocations(t *testing.T) {
	dir := remainingSearchFixtures(t)
	before, err := os.ReadFile(filepath.Join(dir, "work.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	report := searchCLI(t, "-dir", dir, "-grep", "c::", "-json")
	if report.Query != "c::" || len(report.Matches) != 4 {
		t.Fatal(report)
	}
	var got []string
	for _, match := range report.Matches {
		got = append(got, match.ID)
	}
	if !reflect.DeepEqual(got, []string{"library.one", "work.one", "work.one", "work.more"}) {
		t.Fatal("unstable or incorrect match order", got)
	}
	first := report.Matches[1]
	if first.Line != 8 || first.Status != "partial" || first.PointIndex != 1 || first.Remaining != 3 || first.Point != "Bind C::Value with [ref]" {
		t.Fatal("lost checklist location", first)
	}
	for query, want := range map[string]int{"[ref]": 1, "MD-021": 1, "åäö": 1, "rule across line two": 1, "hidden only": 0, "description": 0, "expectation": 0, "C.*": 0} {
		report := searchCLI(t, "-dir", dir, "-grep", query, "-json")
		if report.Matches == nil || len(report.Matches) != want {
			t.Fatalf("query %q: %+v, want %d matches", query, report, want)
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, "work.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("search rewrote input", err)
	}
	if again := searchCLI(t, "-dir", dir, "-grep", "c::", "-json"); !reflect.DeepEqual(report, again) {
		t.Fatal("nondeterministic search", again)
	}
}

func TestRemainingSearchScopeHumanAndEmptyReports(t *testing.T) {
	dir := remainingSearchFixtures(t)
	for _, filter := range [][]string{{"-file", "work.yaml"}, {"-file", filepath.Join(dir, "work.yaml")}, {"-id", "work.one"}} {
		args := append([]string{"-dir", dir, "-grep", "C::", "-json"}, filter...)
		report := searchCLI(t, args...)
		want := 3
		if filter[0] == "-id" {
			want = 2
		}
		if len(report.Matches) != want {
			t.Fatal(filter, report)
		}
	}
	// A positional directory and a fragment passed as -dir are supported too.
	if got := searchCLI(t, "-grep", "MD-021", "-json", dir); len(got.Matches) != 1 {
		t.Fatal(got)
	}
	if got := searchCLI(t, "-dir", filepath.Join(dir, "work.yaml"), "-grep", "C::", "-json"); len(got.Matches) != 3 {
		t.Fatal(got)
	}
	if got := searchCLI(t, "-dir", dir, "-id", "work.done", "-grep", "C::", "-json"); got.Matches == nil || len(got.Matches) != 0 {
		t.Fatal(got)
	}
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-grep", "MD-021"}, &out, &errors); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 remaining points match", "work.yaml:9: work.one [partial] remaining[2/3]", "id: MD-021", "description: Resolve the ÅÄÖ rule across line two"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-grep", "absent"}, &out, &errors); err != nil || out.String() != "0 remaining points match \"absent\"\n" {
		t.Fatal(err, out.String())
	}
}

func TestRemainingSearchRejectsInvalidRequests(t *testing.T) {
	dir := remainingSearchFixtures(t)
	for _, flags := range [][]string{{"-random"}, {"-point", "1", "-id", "work.one"}, {"-exclude", "x"}, {"-uses", "x"}, {"-paths"}, {"-go"}, {"-html"}, {"-itemonly"}, {"-verbose"}, {"-id", "missing"}, {"-file", "library.yaml", "-id", "work.one"}} {
		var out, errors bytes.Buffer
		args := append([]string{"-dir", dir, "-grep", "C::"}, flags...)
		if err := runCLI(args, &out, &errors); err == nil || out.Len() != 0 {
			t.Fatal("accepted incompatible request or emitted partial results", args, err, out.String())
		}
	}
	for _, query := range []string{"", " \t\n"} {
		var out, errors bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-grep", query}, &out, &errors); err == nil || out.Len() != 0 {
			t.Fatal("empty search accepted", err)
		}
	}
	writeFixture(t, dir, "duplicate.yaml", "id: work.one\nstatus: implemented\n")
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-id", "work.one", "-grep", "C::"}, &out, &errors); err == nil || out.Len() != 0 {
		t.Fatal("ambiguous owner accepted", err)
	}
}
