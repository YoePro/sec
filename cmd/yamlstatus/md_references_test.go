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

func decisionReferenceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "governance/one.yaml", "id: example\nstatus: partial\nremaining:\n  - blocked on MD-012\n")
	writeFixture(t, root, "missing-decisions.yaml", "decisions:\n  - id: MD-012\n    status: open\n  - id: MD-034\n    status: decided-implementation-pending\n")
	return root, filepath.Join(root, "governance")
}

// Exact IDs find every occurrence in the documented source universe, including
// comments and historical text, with portable paths and Unicode source columns.
// The query is read-only and does not follow external source symlinks.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func TestDecisionReferencesExact(t *testing.T) {
	root, dir := decisionReferenceFixture(t)
	writeFixture(t, root, "rules/book.md", "å MD-012 then MD-012\nnot MD-0123 or XMD-012 or MD-012suffix\n")
	writeFixture(t, root, "language-rulebook-status.md", "MD-012\n")
	writeFixture(t, root, "internal/example.go", "// MD-012\n")
	writeFixture(t, root, "sec/example.sec", "// MD-012\n")
	writeFixture(t, root, "bootstrap/example.sec", "// MD-012\n")
	writeFixture(t, root, "unrelated.md", "MD-012\n")
	writeFixture(t, root, "build/generated.go", "// MD-012\n")
	writeFixture(t, root, "rules/image.bin", "\x00MD-012\n")
	external := filepath.Join(t.TempDir(), "external.go")
	if err := os.WriteFile(external, []byte("// MD-012\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "internal", "link.go")); err != nil {
		t.Fatal(err)
	}
	report, err := findDecisionReferences(root, dir, "MD-012")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, file := range report.Files {
		paths = append(paths, file.File)
	}
	want := []string{"bootstrap/example.sec", "governance/one.yaml", "internal/example.go", "language-rulebook-status.md", "rules/book.md", "sec/example.sec"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("files = %v", paths)
	}
	refs := report.Files[4].References
	if len(refs) != 2 || refs[0].Line != 1 || refs[0].Column != 3 || refs[1].Column != 15 {
		t.Fatalf("references = %+v", refs)
	}
	var first, second bytes.Buffer
	if err := printDecisionReferences(&first, report, true); err != nil {
		t.Fatal(err)
	}
	again, err := findDecisionReferences(root, dir, "MD-012")
	if err != nil {
		t.Fatal(err)
	}
	if err := printDecisionReferences(&second, again, true); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatal("nondeterministic output")
	}
	data, err := os.ReadFile(filepath.Join(dir, "one.yaml"))
	if err != nil || !bytes.Contains(data, []byte("blocked on MD-012")) {
		t.Fatal("query changed input")
	}
}

// Closed mode means absent from the decision ledger, with explicit potential
// blocker wording. It does not confuse implementation-pending with closed,
// classify historical neutral mentions, or claim to prove normative status.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func TestDecisionReferencesClosed(t *testing.T) {
	root, dir := decisionReferenceFixture(t)
	writeFixture(t, root, "rules/closed.md", "blocked on MD-099\nawait\nMD-100\nMD-101 remains undecided\nMD-102 is open\nMD-103 (open)\nMD-104 applied yesterday\nnot blocked on MD-105\nno longer awaiting MD-106\nMD-107 is openers\nblocked on MD-034\n")
	report, err := findDecisionReferences(root, dir, "closed")
	if err != nil {
		t.Fatal(err)
	}
	if !report.PossibleStaleBlockers || len(report.Files) != 1 {
		t.Fatalf("report = %+v", report)
	}
	var ids []string
	for _, ref := range report.Files[0].References {
		ids = append(ids, ref.ID)
	}
	if !reflect.DeepEqual(ids, []string{"MD-099", "MD-100", "MD-101", "MD-102", "MD-103"}) {
		t.Fatalf("IDs = %v", ids)
	}
	if report.Files[0].References[1].Line != 3 {
		t.Fatalf("wrapped line = %+v", report.Files[0].References[1])
	}
	var text bytes.Buffer
	if err := printDecisionReferences(&text, report, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "Possible stale blocker") || strings.Contains(text.String(), root) {
		t.Fatal(text.String())
	}
}

// The CLI exposes only compatible read-only options, validates the ledger,
// preserves valid empty results and reports malformed input instead of guessing.
// Contract: cmd/yamlstatus/IDEAS.md §5.
func TestDecisionReferencesCLI(t *testing.T) {
	root, dir := decisionReferenceFixture(t)
	for _, query := range []string{"MD-012", "MD-999", "closed"} {
		var out, errOut bytes.Buffer
		if err := runCLI([]string{"-dir", dir, "-md", query, "-json"}, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		var report DecisionReferenceReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Query != query {
			t.Fatalf("%v: %s", err, out.String())
		}
	}
	for _, args := range [][]string{
		{"-md", ""}, {"-md", "MD-12"}, {"-md", "MD-012extra"},
		{"-md", "MD-012", "-grep", "x"}, {"-md", "closed", "-file", "one.yaml"},
		{"-md", "MD-012", "-listfiles"}, {"-md", "closed", "-snapshot", "x"},
		{"-md", "MD-012", "-paths"}, {"-md", "closed", "-verbose=false"},
	} {
		var out, errOut bytes.Buffer
		if err := runCLI(append([]string{"-dir", dir}, args...), &out, &errOut); err == nil || out.Len() != 0 {
			t.Fatalf("accepted %v: %s", args, out.String())
		}
	}
	for _, bad := range []string{"decisions: {}\n", "decisions:\n - id: wrong\n", "decisions:\n - id: MD-012\n - id: MD-012\n", "decisions: []\ndecisions: []\n"} {
		writeFixture(t, root, "missing-decisions.yaml", bad)
		if _, err := findDecisionReferences(root, dir, "closed"); err == nil {
			t.Fatalf("accepted ledger %q", bad)
		}
	}
}
