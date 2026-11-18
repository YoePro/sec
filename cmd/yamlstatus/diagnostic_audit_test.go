package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func diagnosticAuditFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "governance/errors_diagnostics.yaml", "id: diagnostics.example\nstatus: partial\nimplemented: ['S1001; retired S1002']\n")
	return root, filepath.Join(root, "governance")
}

// The canonical registry, including retired identities, is compared against
// exact textual mentions in every selected fragment and rulebook. Rulebook-only
// mentions do not pretend an ID is documented in governance. Source locations,
// Unicode columns, ordering and topic-family metadata are preserved.
// Contract: cmd/yamlstatus/IDEAS.md §8; rules/tooling/diagnostics.md §§3,5–6.
func TestDiagnosticAuditReferences(t *testing.T) {
	root, dir := diagnosticAuditFixture(t)
	writeFixture(t, root, "governance/other.yaml", "# å S1004 S1004; S10040 XS1004 S1004suffix\r\nid: other\r\nstatus: partial\r\n")
	writeFixture(t, root, "rules/book.md", "Registered S1003, reserved proposal S1005.\n")
	writeFixture(t, root, "unrelated.md", "S1006\n")
	outside := t.TempDir()
	writeFixture(t, outside, "external.md", "S1006\n")
	if err := os.Symlink(filepath.Join(outside, "external.md"), filepath.Join(root, "rules", "link.md")); err != nil {
		t.Fatal(err)
	}
	definitions := []diagnostics.Definition{
		{ID: "S1003", Name: "types.three", Family: "parser"},
		{ID: "S1001", Name: "ownership.one", Family: "ownership"},
		{ID: "S1002", Name: "ownership.two", Family: "ownership", Retired: true},
	}
	before, err := os.ReadFile(filepath.Join(dir, "other.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := auditDiagnosticRegistry(root, dir, definitions)
	if err != nil {
		t.Fatal(err)
	}
	if report.Registered != 3 || len(report.UnregisteredMentions) != 3 || len(report.UnmentionedInGovernance) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.UnmentionedInGovernance[0].ID != "S1003" || report.UnmentionedInGovernance[0].TopicFamily != "parser" {
		t.Fatal(report.UnmentionedInGovernance)
	}
	for i, col := range []int{5, 11} {
		ref := report.UnregisteredMentions[i]
		if ref.File != "governance/other.yaml" || ref.ID != "S1004" || ref.Line != 1 || ref.Column != col || strings.ContainsRune(ref.Text, '\r') {
			t.Fatalf("reference = %+v", ref)
		}
	}
	candidates := map[string]DiagnosticNamespaceCandidate{}
	for _, candidate := range report.Candidates {
		candidates[candidate.Namespace] = candidate
	}
	if candidates["S"].NextID != "S1006" || candidates["S"].Retired != 1 || candidates["S"].Registered != 3 {
		t.Fatal(candidates)
	}
	if candidates["P"].Registered != 0 || candidates["P"].NextID != "P0000" {
		t.Fatal("topic family became namespace", candidates)
	}
	// Reversing registration order cannot change the audit or its suggestions.
	reversed := append([]diagnostics.Definition(nil), definitions...)
	reversed[0], reversed[2] = reversed[2], reversed[0]
	again, err := auditDiagnosticRegistry(root, dir, reversed)
	if err != nil || !reflect.DeepEqual(report, again) {
		t.Fatalf("unstable: %v / %+v", err, again)
	}
	after, err := os.ReadFile(filepath.Join(dir, "other.yaml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("audit changed source")
	}
}

// Suggestions never recycle retired IDs or gaps and reserve unregistered prose
// references. Exhausted append space is explicit; no numeric topic subranges
// or new diagnostic namespaces are inferred.
// Contract: cmd/yamlstatus/IDEAS.md §8; rules/tooling/diagnostics.md §5(4–10).
func TestDiagnosticAuditCandidatesAndValidation(t *testing.T) {
	root, dir := diagnosticAuditFixture(t)
	writeFixture(t, root, "rules/book.md", "S9999 remains reserved in a proposal.\n")
	report, err := auditDiagnosticRegistry(root, dir, []diagnostics.Definition{{ID: "S9998", Retired: true}, {ID: "L0000"}})
	if err != nil {
		t.Fatal(err)
	}
	candidates := map[string]DiagnosticNamespaceCandidate{}
	for _, candidate := range report.Candidates {
		candidates[candidate.Namespace] = candidate
	}
	if !candidates["S"].AppendSpaceExhausted || candidates["S"].NextID != "" || candidates["L"].NextID != "L0001" {
		t.Fatal(candidates)
	}
	if len(report.UnmentionedInGovernance) != 2 || !report.UnmentionedInGovernance[1].Retired {
		t.Fatal(report.UnmentionedInGovernance)
	}
	for _, bad := range [][]diagnostics.Definition{
		{{ID: "Z1001"}}, {{ID: "S123"}}, {{ID: "S12345"}}, {{ID: "S1001"}, {ID: "S1001", Retired: true}},
	} {
		if _, err := auditDiagnosticRegistry(root, dir, bad); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
	writeFixture(t, root, "rules/book.md", string([]byte{0xff}))
	if _, err := auditDiagnosticRegistry(root, dir, nil); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

// The CLI consumes diagnostics.All rather than parsing registry source text,
// supports portable plain/JSON output and rejects conflicts before output.
// Findings are read-only audit data, not an automatic allocation or source error.
// Contract: cmd/yamlstatus/IDEAS.md §8.
func TestDiagnosticAuditCLI(t *testing.T) {
	root, dir := diagnosticAuditFixture(t)
	t.Chdir(root)
	for _, args := range [][]string{{"-diagnostics", "-json"}, {"-dir", dir, "-diagnostics", "-json"}, {"-diagnostics", "-json", dir}} {
		var out, errOut bytes.Buffer
		if err := runCLI(args, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		var report DiagnosticAudit
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Registered != len(diagnostics.All()) || len(report.Candidates) != 6 {
			t.Fatalf("%v: %s", err, out.String())
		}
	}
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"-diagnostics"}, &out, &errOut); err != nil || !strings.Contains(out.String(), "next candidate") || strings.Contains(out.String(), root) {
		t.Fatalf("%v: %s", err, out.String())
	}
	for _, option := range []string{"-corrections", "-md=closed", "-file=errors_diagnostics.yaml", "-id=x", "-listfiles", "-paths", "-grep=x", "-snapshot=x", "-verbose=false", "-html"} {
		out.Reset()
		if err := runCLI([]string{"-diagnostics", option}, &out, &errOut); err == nil || out.Len() != 0 {
			t.Fatalf("accepted %s", option)
		}
	}
	writeFixture(t, root, "governance/errors_diagnostics.yaml", "id: duplicate\nid: duplicate\n")
	out.Reset()
	if err := runCLI([]string{"-diagnostics"}, &out, &errOut); err == nil || out.Len() != 0 {
		t.Fatal("accepted malformed governance")
	}
}
