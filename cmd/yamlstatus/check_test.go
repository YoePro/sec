package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const checkedFragment = "schema_version: 1\narea: core\npurpose: Core.\nintegrations:\n" +
	"  - id: core.one\n" +
	"    status: partial\n" +
	"    implemented:\n" +
	"      - plain item\n" +
	"      - id: keyed\n" +
	"        description: keyed item\n" +
	"    required_tests:\n" +
	"      - id: keyed-test\n" +
	"        expectation: rejects x\n" +
	"    remaining:\n" +
	"      - first\n" +
	"    verification:\n" +
	"      - command: go test ./...\n" +
	"        result: passed\n"

func TestCheckAcceptsWellFormedLedger(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": checkedFragment})
	writeFixture(t, dir, "index.yaml", "schema_version: 1\nstatus_values:\n  partial: some\n  planned: none\nfragments:\n  - path: core.yaml\n")
	out, err := runCommand(t, "check", "-dir", dir)
	if err != nil || !strings.Contains(out, "0 problems in 2 fragments") {
		t.Fatalf("check = %v\n%s", err, out)
	}
}

func TestCheckReportsEveryStructuralProblem(t *testing.T) {
	broken := "schema_version: 1\narea: core\nintegrations:\n" +
		"  - id: core.one\n" +
		"    status: finished\n" +
		"    implemented:\n" +
		"      - text with: a colon\n" +
		"      - first item that swallowed      - \"the next item\"\n" +
		"      - command: go test\n" +
		"        result: passed\n" +
		"      - 2.0\n" +
		"    remaining:\n" +
		"    verification:\n" +
		"      - go test ./... passed\n" +
		"      - command: go vet\n" +
		"\n" +
		"  - id: core.two\n" +
		"    implemented: []\n"
	dir := governanceFixture(t, map[string]string{
		"core.yaml":  broken,
		"other.yaml": "schema_version: 1\narea: other\npurpose: Other.\nintegrations:\n  - id: core.two\n    status: planned\n",
		"extra.yaml": "schema_version: 1\narea: extra\npurpose: Extra.\nintegrations: []\n",
		"dup.yaml":   "a: 1\na: 2\n",
	})
	writeFixture(t, dir, "index.yaml", "schema_version: 1\nstatus_values:\n  partial: some\n  planned: none\nfragments:\n  - path: core.yaml\n  - path: other.yaml\n  - path: gone.yaml\n")
	out, err := runCommand(t, "check", "-dir", dir)
	if err == nil {
		t.Fatalf("check passed a broken ledger\n%s", out)
	}
	for _, want := range []string{
		`dup.yaml: (fragment): does not load: :2: duplicate key "a" (first defined at line 1)`,
		"core.yaml:1: (fragment): fragment header has no purpose",
		`core.one: status "finished" is not one of partial, planned`,
		`core.one: implemented item 1 "text with" parses as a one-pair mapping because its text contains ": "; quote it (yamlstatus fmt -fix-types)`,
		`core.one: implemented item 2 contains a list marker`,
		"core.one: implemented item 3 is a mapping with keys command, result",
		"core.one: implemented item 4 is the float 2.0, not text",
		"core.one: remaining is a scalar, not a list",
		`core.one: verification item 1 must be a mapping with exactly a command and a result, got the text "go test ./... passed"`,
		"core.one: verification item 2 must be a mapping with exactly a command and a result, got keys command",
		"core.yaml:17: core.two: integration has no status",
		"other.yaml:5: core.two: integration ID is also defined at " + filepath.Join(dir, "core.yaml") + ":17",
		"extra.yaml: (fragment): fragment is not listed in index.yaml",
		"index.yaml: (fragment): listed fragment gone.yaml does not exist",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if t.Failed() {
		t.Log(out)
	}
}

func TestEditsRefuseToIntroduceProblems(t *testing.T) {
	dir := governanceFixture(t, map[string]string{"core.yaml": checkedFragment})
	path := filepath.Join(dir, "core.yaml")
	for _, args := range [][]string{
		{"move", "-dir", dir, "-write", "core.one", "verification", "1", "implemented"},
		{"add", "-dir", dir, "-write", "core.one", "verification", "go test passed"},
	} {
		if _, err := runCommand(t, args...); err == nil || !strings.Contains(err.Error(), "would introduce governance problems") {
			t.Errorf("%v: error = %v", args, err)
		}
	}
	if readFile(t, path) != checkedFragment {
		t.Fatal("a refused edit changed the fragment")
	}
	// Problems the fragment already had do not block unrelated edits.
	dir = governanceFixture(t, map[string]string{"core.yaml": canonicalFragment})
	if out, err := runCommand(t, "add", "-dir", dir, "-write", "core.one", "remaining", "third"); err != nil {
		t.Fatalf("edit of a fragment without purpose = %v\n%s", err, out)
	}
}
