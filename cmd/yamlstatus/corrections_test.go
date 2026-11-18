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

// Every listed application has a real archive target; archive links may come
// from prose or a rulebook, but an archive cannot establish its own linkage.
// Repeated valid links remain valid. Reports never move or rewrite files.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func TestCorrectionAuditLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	writeFixture(t, root, "governance/one.yaml", `integrations:
  - id: first
    status: partial
    corrections_applied:
      - rules/corrections/applied/listed.md
      - rules/corrections/applied/missing.md
      - rules/corrections/pending.md
      - rules/corrections/applied/../../escape.md
    implemented:
      - See rules/corrections/applied/prose.md for details.
  - id: second
    status: partial
    corrections_applied:
      - rules/corrections/applied/listed.md
`)
	for _, name := range []string{"listed.md", "prose.md", "book.md", "orphan.md", "prefix.md", "relative file.md"} {
		path := "rules/corrections/applied/" + name
		writeFixture(t, root, path, "# Archived correction\n- **Status:** Applied\nSelf: "+path+"\n")
	}
	writeFixture(t, root, "rules/book.md", "See [change](rules/corrections/applied/book.md#part).\nDo not count rules/corrections/applied/prefix.md.extra\n")
	writeFixture(t, root, "rules/memory/book.md", "[relative](../corrections/applied/relative%20file.md#part)\n[remote](https://example.org/rules/corrections/applied/orphan.md)\n")
	report, err := auditCorrections(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.CheckedLinks != 5 || report.ArchivedFiles != 6 || len(report.BrokenLinks) != 3 {
		t.Fatalf("report = %+v", report)
	}
	if report.BrokenLinks[0].ID != "first" || report.BrokenLinks[0].Line != 6 {
		t.Fatalf("location = %+v", report.BrokenLinks[0])
	}
	want := []string{"rules/corrections/applied/orphan.md", "rules/corrections/applied/prefix.md"}
	if !reflect.DeepEqual(report.UnreferencedApplied, want) {
		t.Fatal(report.UnreferencedApplied)
	}
	before, err := os.ReadFile(filepath.Join(dir, "one.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := auditCorrections(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report, again) {
		t.Fatal("nondeterministic report")
	}
	after, err := os.ReadFile(filepath.Join(dir, "one.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("audit changed governance")
	}
}

// Normative application and pending runtime implementation are distinct. Root
// candidates need correction evidence, and code examples or nested statuses
// cannot mark the document applied. Symlinks do not escape the archive scope.
// Contract: cmd/yamlstatus/IDEAS.md §6; rules/corrections/README.md.
func TestCorrectionAuditPendingAndSymlinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	writeFixture(t, root, "governance/one.yaml", `id: example
status: partial
corrections_applied:
  - rules/corrections/applied/link.md
  - rules/corrections/applied/linked-dir/outside.md
`)
	writeFixture(t, root, "rules/corrections/pending.md", "# Fix\n- **Status:** Pending\n")
	writeFixture(t, root, "rules/corrections/applied-later.md", "# Correction\n- **Status:** Applied\n")
	writeFixture(t, root, "rules/corrections/yaml-status.yaml", "---\nstatus: Applied\n")
	writeFixture(t, root, "rules/corrections/README.md", "# Rulebook corrections\n")
	writeFixture(t, root, "rules/corrections/nested.yml", "schema_version: 1\nintegrations:\n  - status: applied\n")
	writeFixture(t, root, "root-correction.md", "# New correction\n- **Status:** Decided; implementation pending\n")
	writeFixture(t, root, "named-fix.md", "# Correction for X\n## Example\nstatus: Applied\n")
	writeFixture(t, root, "proposal.md", "# An unrelated proposal\nDiscusses future corrections.\n")
	outside := t.TempDir()
	writeFixture(t, outside, "outside.md", "- **Status:** Applied\n")
	if err := os.MkdirAll(filepath.Join(root, "rules/corrections/applied"), 0755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{"link.md": filepath.Join(outside, "outside.md"), "linked-dir": outside} {
		if err := os.Symlink(target, filepath.Join(root, "rules/corrections/applied", link)); err != nil {
			t.Fatal(err)
		}
	}
	report, err := auditCorrections(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.ArchivedFiles != 0 || len(report.BrokenLinks) != 2 || len(report.Pending) != 4 || len(report.AwaitingArchive) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if report.Pending[0].Status != "unspecified" || report.Pending[1].Status != "Decided; implementation pending" {
		t.Fatal(report.Pending)
	}
	if report.AwaitingArchive[0].File != "rules/corrections/applied-later.md" {
		t.Fatal(report.AwaitingArchive)
	}
}

// The read-only audit supports plain/JSON output and both directory forms.
// Invalid governance and mode conflicts fail before producing partial reports.
// Findings are reported without implying implementation completion or editing.
// Contract: cmd/yamlstatus/IDEAS.md §6.
func TestCorrectionAuditCLI(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "governance")
	writeFixture(t, root, "governance/one.yaml", "id: example\nstatus: partial\n")
	writeFixture(t, root, "rules/corrections/pending.md", "# Correction\nStatus: Pending\n")
	t.Chdir(root)
	for _, args := range [][]string{{"-dir", dir, "-corrections", "-json"}, {"-corrections", "-json", dir}, {"-corrections", "-json"}} {
		var out, errOut bytes.Buffer
		if err := runCLI(args, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		var report CorrectionAudit
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || len(report.Pending) != 1 {
			t.Fatalf("%v: %s", err, out.String())
		}
	}
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"-dir", dir, "-corrections"}, &out, &errOut); err != nil || !strings.Contains(out.String(), "pending: rules/corrections/pending.md") || strings.Contains(out.String(), root) {
		t.Fatalf("%v: %s", err, out.String())
	}
	for _, flag := range []string{"-paths", "-md=closed", "-grep=x", "-snapshot=x", "-file=one.yaml", "-id=example", "-listfiles", "-verbose=false", "-html"} {
		out.Reset()
		if err := runCLI([]string{"-dir", dir, "-corrections", flag}, &out, &errOut); err == nil || out.Len() != 0 {
			t.Fatalf("accepted %s", flag)
		}
	}
	for _, bad := range []string{"id: x\nstatus: partial\ncorrections_applied: wrong\n", "id: x\nstatus: partial\ncorrections_applied: [1]\n", "id: x\nstatus: partial\ncorrections_applied: ['']\n"} {
		writeFixture(t, root, "governance/one.yaml", bad)
		if _, err := auditCorrections(root, dir); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
