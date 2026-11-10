package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCountSnapshotDiff exercises a real baseline and subsequent count, scope,
// status and ownership changes, including completed and library integrations.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func TestCountSnapshotDiff(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "index.yaml", "fragments:\n  - path: work.yaml\n  - path: library.yaml\n    implementation_scope: library\n")
	original := "integrations:\n  - id: work.one\n    status: partial\n    implemented: [old]\n    remaining: [first, second]\n    partial: [unfinished]\n  - id: work.removed\n    status: planned\n    remaining: [lost]\n  - id: work.move\n    status: implemented\n    implemented: [done]\n"
	writeFixture(t, dir, "work.yaml", original)
	writeFixture(t, dir, "library.yaml", "id: library.one\nstatus: planned\nremaining: [not compiler]\n")
	baseline := filepath.Join(t.TempDir(), "counts.json")
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-snapshot", baseline}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(dir)) {
		t.Fatal("snapshot contains private path")
	}
	if err := runCLI([]string{"-dir", dir, "-snapshot", baseline}, &out, &errOut); err == nil {
		t.Fatal("overwrote existing snapshot")
	}
	writeFixture(t, dir, "work.yaml", "integrations:\n  - id: work.one\n    status: implemented\n    implemented: [old, first, second]\n    remaining: []\n    partial: [unfinished]\n  - id: work.added\n    status: planned\n    remaining: [new]\n")
	writeFixture(t, dir, "moved.yaml", "id: work.move\nstatus: implemented\nimplemented: [done]\n")
	writeFixture(t, dir, "library.yaml", "id: library.one\nstatus: implemented\nimplemented: [done]\n")
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-diff", baseline, "-json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var report countDiff
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Integrations) != 5 || len(report.Totals) != 2 {
		t.Fatal(out.String())
	}
	compiler, library := report.Totals[0], report.Totals[1]
	if compiler.Scope != "compiler" || compiler.Before.Remaining != 3 || compiler.After.Remaining != 1 || compiler.Delta.Implemented != 2 || compiler.Delta.Partial != 0 {
		t.Fatal(compiler)
	}
	if library.Scope != "library" || library.Delta.Remaining != -1 || library.Delta.Implemented != 1 {
		t.Fatal(library)
	}
	for i := 1; i < len(report.Integrations); i++ {
		if report.Integrations[i-1].ID >= report.Integrations[i].ID {
			t.Fatal("not deterministic")
		}
	}
	first := out.String()
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-diff", baseline, "-json"}, &out, &errOut); err != nil || out.String() != first {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-diff", baseline}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"remaining 3 -> 1 (-2)", "work.added [added]", "work.removed [removed]", "work.move [moved: work.yaml -> moved.yaml]", "status: partial -> implemented"} {
		if !strings.Contains(out.String(), text) {
			t.Fatal(text, out.String())
		}
	}
	preserved, err := os.ReadFile(baseline)
	if err != nil || !bytes.Equal(data, preserved) {
		t.Fatal("comparison modified baseline", err)
	}
}

// TestCountDiffHEAD compares committed YAML with uncommitted additions,
// deletions and edits without touching the Git index or working tree.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func TestCountDiffHEAD(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		args = append([]string{"-C", root}, args...)
		data, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(data))
		}
		return string(data)
	}
	git("init", "-q")
	writeFixture(t, dir, "old.yaml", "id: old\nstatus: planned\nremaining: [one, two]\n")
	git("add", ".")
	git("-c", "user.name=Count Test", "-c", "user.email=count@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "baseline")
	if err := os.Remove(filepath.Join(dir, "old.yaml")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "new.yaml", "id: added\nstatus: planned\nremaining: [one]\n")
	status := git("status", "--porcelain")
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-diff", "HEAD", "-json"}, &out, &errors); err != nil {
		t.Fatal(err, errors.String())
	}
	var report countDiff
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Integrations) != 2 || report.Totals[0].Delta.Remaining != -1 {
		t.Fatal(out.String())
	}
	if status != git("status", "--porcelain") {
		t.Fatal("HEAD comparison changed working tree/index")
	}
	if strings.Contains(out.String(), root) {
		t.Fatal("comparison exposes private path")
	}
}

// TestCountDiffInvalidInputs keeps malformed or incompatible snapshots and CLI
// combinations from silently producing plausible comparison results.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func TestCountDiffInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "one.yaml", "id: one\nstatus: partial\nremaining: [one]\n")
	for _, args := range [][]string{{"-snapshot", ""}, {"-diff", ""}, {"-snapshot", "x", "-diff", "HEAD"}, {"-diff", "HEAD", "-file", ""}, {"-snapshot", "x", "-json"}, {"-diff", "HEAD", "-grep", ""}} {
		var out, errors bytes.Buffer
		if err := runCLI(append([]string{"-dir", dir}, args...), &out, &errors); err == nil {
			t.Fatal(args)
		}
	}
	entry := `{"id":"x","file":"one.yaml","scope":"compiler","status":"partial","counts":{"remaining":1,"implemented":0,"partial":0}}`
	for _, data := range []string{`{}`, `{"schema_version":2,"entries":[]}`, `{"schema_version":1,"entries":[]} {}`, `{"schema_version":1,"entries":[],"unknown":0}`, `{"schema_version":1,"entries":[` + entry + `,` + entry + `]}`, strings.Replace(`{"schema_version":1,"entries":[`+entry+`]}`, `one.yaml`, `../one.yaml`, 1), strings.Replace(`{"schema_version":1,"entries":[`+entry+`]}`, `"remaining":1`, `"remaining":-1`, 1)} {
		file := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errors bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-diff", file}, &out, &errors); err == nil {
			t.Fatal(data)
		}
	}
	writeFixture(t, dir, "duplicate.yaml", "id: one\nstatus: partial\n")
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-snapshot", filepath.Join(t.TempDir(), "new.json")}, &out, &errors); err == nil {
		t.Fatal("duplicate owner accepted")
	}
}

// TestCountDiffScopeAndEmpty preserves scope reclassification even when an
// integration's counts stay the same, and emits empty arrays for no changes.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func TestCountDiffScopeAndEmpty(t *testing.T) {
	entry := countEntry{ID: "one", File: "one.yaml", Scope: "library", Status: "partial", Counts: checklistCounts{Remaining: 2}}
	before := countSnapshot{SchemaVersion: 1, Entries: []countEntry{entry}}
	entry.Scope = "compiler"
	after := countSnapshot{SchemaVersion: 1, Entries: []countEntry{entry}}
	report := compareCounts(before, after)
	if len(report.Integrations) != 1 || len(report.Fragments) != 2 || len(report.Totals) != 2 || report.Integrations[0].Delta.Remaining != 0 {
		t.Fatal(report)
	}
	if report.Totals[0].Delta.Remaining != 2 || report.Totals[1].Delta.Remaining != -2 {
		t.Fatal(report.Totals)
	}
	unchanged := compareCounts(after, after)
	if len(unchanged.Integrations) != 0 || len(unchanged.Fragments) != 0 {
		t.Fatal(unchanged)
	}
	var output bytes.Buffer
	if err := printCountDiff(&output, unchanged, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"integrations": []`) {
		t.Fatal(output.String())
	}
}
