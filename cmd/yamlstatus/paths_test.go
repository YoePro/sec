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

func TestMissingPathsReportsEveryLocationWithoutWriting(t *testing.T) {
	root, dir := usesFixtures(t)
	writeFixture(t, root, "internal/shared.go", "package shared\n")
	writeFixture(t, root, "rules/existing.md", "existing\n")
	writeFixture(t, dir, "nested.yaml", `integrations:
  - id: nested.one
    status: partial
    evidence:
      code: [cmd/missing.go]
      tests: [tests/missing.sec]
    plans:
      - rules: [rules/existing.md, rules/missing.md]
        code: ['internal\\shared.go']
    rules: [{id: rule.one, requirement: not-a-path}, '§ 4(1)']
`)
	before, err := os.ReadFile(filepath.Join(dir, "nested.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err = runCLI([]string{"-dir", dir, "-paths", "-json"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "missing path references") {
		t.Fatalf("missing references should fail the check: %v", err)
	}
	var report MissingPaths
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err, out.String())
	}
	// Repeated references retain locations; prose, requirements and existing
	// normalized paths are excluded, but completed/library owners are checked.
	if report.Checked != 11 || len(report.Missing) != 5 {
		t.Fatalf("unexpected report: %+v", report)
	}
	want := []string{"core.done:tests:internal/shared.go.bak", "docs.other:code:internal/other.go", "nested.one:evidence.code:cmd/missing.go", "nested.one:evidence.tests:tests/missing.sec", "nested.one:plans[0].rules:rules/missing.md"}
	var got []string
	for _, ref := range report.Missing {
		if ref.Line <= 0 || ref.File == "" || ref.Status == "" {
			t.Fatal("missing YAML navigation metadata", ref)
		}
		got = append(got, ref.ID+":"+ref.Field+":"+ref.Path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	after, err := os.ReadFile(filepath.Join(dir, "nested.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("query changed governance", err)
	}
	// Deterministic JSON and source-order locations across repeated queries.
	var repeated bytes.Buffer
	_ = runCLI([]string{"-dir", dir, "-paths", "-json"}, &repeated, &errOut)
	if repeated.String() != out.String() {
		t.Fatal("unstable report")
	}
}

func TestMissingPathsGoFilterAndScope(t *testing.T) {
	root, dir := usesFixtures(t)
	writeFixture(t, dir, "filter.yaml", `id: filtered
status: implemented
code: [cmd/gone.go, internal/gone.go, internal/nested/gone.go, other/gone.go, internal/not-go.go.bak, internalized/no.go]
tests: [testdata/missing.sec]
`)
	for _, scope := range [][]string{{"-id", "filtered"}, {"-file", "filter.yaml"}, {"-file", filepath.Join(dir, "filter.yaml")}} {
		args := append([]string{"-dir", dir, "-paths", "-go", "-json"}, scope...)
		var out, errors bytes.Buffer
		if err := runCLI(args, &out, &errors); err == nil {
			t.Fatal("missing paths accepted")
		}
		var report MissingPaths
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Checked != 3 || len(report.Missing) != 3 {
			t.Fatal(report, err, out.String())
		}
	}
	writeFixture(t, dir, "absolute.yaml", "id: absolute\nstatus: implemented\ncode: ["+filepath.ToSlash(filepath.Join(root, "cmd/gone.go"))+"]\n")
	var human, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-paths", "-go", "-id", "absolute"}, &human, &errors); err == nil ||
		!strings.Contains(human.String(), "absolute.yaml:3: absolute [implemented] code:") ||
		!strings.Contains(human.String(), "1 missing path references; 1 checked") {
		t.Fatal(err, human.String())
	}
}

func TestMissingPathsSuccessAndSymlinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	writeFixture(t, root, "cmd/exists.go", "package main\n")
	writeFixture(t, dir, "one.yaml", "id: one\nstatus: implemented\ncode: [./cmd/exists.go]\nrules: [{id: rule, requirement: prose}]\n")
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-paths", "-json"}, &out, &errors); err != nil {
		t.Fatal(err)
	}
	var report MissingPaths
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Checked != 1 || report.Missing == nil || len(report.Missing) != 0 {
		t.Fatal(report, err)
	}
	// When -dir names a fragment, paths still resolve from its parent's parent.
	out.Reset()
	if err := runCLI([]string{"-paths", filepath.Join(dir, "one.yaml")}, &out, &errors); err != nil || out.String() != "0 missing path references; 1 checked\n" {
		t.Fatal(err, out.String())
	}
	if err := os.Symlink(filepath.Join(root, "cmd/exists.go"), filepath.Join(root, "cmd/link.go")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(filepath.Join(root, "cmd/absent.go"), filepath.Join(root, "cmd/broken.go")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "links.yaml", "id: links\nstatus: partial\ncode: [cmd/link.go, cmd/broken.go]\n")
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-id", "links", "-paths"}, &out, &errors); err == nil || !strings.Contains(out.String(), "cmd/broken.go (missing)") || strings.Contains(out.String(), "cmd/link.go (missing)") {
		t.Fatal(err, out.String())
	}
}

func TestMissingPathsRejectsInvalidRequestsAndLists(t *testing.T) {
	_, dir := usesFixtures(t)
	for _, extra := range [][]string{{"-random"}, {"-point", "1", "-id", "core.done"}, {"-exclude", "x"}, {"-uses", "x"}, {"-html"}, {"-verbose"}, {"-itemonly"}, {"-id", "unknown"}} {
		args := append([]string{"-dir", dir, "-paths"}, extra...)
		var out, errors bytes.Buffer
		if err := runCLI(args, &out, &errors); err == nil || out.Len() != 0 {
			t.Fatal("invalid query accepted or partial output", args, err, out.String())
		}
	}
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-go"}, &out, &errors); err == nil {
		t.Fatal("-go without -paths accepted")
	}
	for _, content := range []string{"code: scalar", "tests: [1]", "code: ['']", "rules: [{requirement: missing-id}]"} {
		writeFixture(t, dir, "bad.yaml", "id: bad\nstatus: partial\n"+content+"\n")
		out.Reset()
		if err := runCLI([]string{"-dir", dir, "-paths"}, &out, &errors); err == nil || out.Len() != 0 {
			t.Fatal("invalid list accepted", content, err, out.String())
		}
	}
	writeFixture(t, dir, "bad.yaml", "id: core.done\nstatus: partial\ncode: []\n")
	if err := runCLI([]string{"-dir", dir, "-paths", "-id", "core.done"}, &out, &errors); err == nil {
		t.Fatal("ambiguous ID accepted")
	}
}

func TestMissingPathsIncludesLibraryOwnersAndInspectionErrors(t *testing.T) {
	root, dir := usesFixtures(t)
	var out, errors bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-paths", "-id", "library.one"}, &out, &errors); err == nil ||
		!strings.Contains(out.String(), "library.one [planned] code: ./internal/shared.go (missing)") {
		t.Fatal("library references were omitted", err, out.String())
	}
	writeFixture(t, root, "not-a-directory", "file\n")
	writeFixture(t, dir, "broken.yaml", "id: broken\nstatus: partial\ncode: [not-a-directory/child.go]\n")
	out.Reset()
	if err := runCLI([]string{"-dir", dir, "-paths", "-id", "broken"}, &out, &errors); err == nil || out.Len() != 0 || !strings.Contains(err.Error(), "not-a-directory/child.go") {
		t.Fatal("inspection failure misreported as missing", err, out.String())
	}
}
