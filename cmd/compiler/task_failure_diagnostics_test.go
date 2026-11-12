package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaskFailureDomainDiagnosticsCLI keeps all five failure/completion
// categories in shared human/JSON diagnostics and rejects invalid artifacts.
// Rules: rules/concurrency/tasks.md — §30(1),(8–9);
// rules/tooling/diagnostics.md — §14.
func TestTaskFailureDomainDiagnosticsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/task_failure_domains_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	invalid := "../../testdata/sema/task_failure_domains_invalid.sec"
	_, output, code = runCLIForDiagnostics(t, "check", invalid)
	if code != 3 || !strings.Contains(output, "S1234") || !strings.Contains(output, "Completed(Err(E))") {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", invalid, "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 21 || len(doc.Occurrences) != 21 {
		t.Fatal(output)
	}
	for _, occurrence := range doc.Occurrences {
		if occurrence.ID == nil || (*occurrence.ID != "S1234" && *occurrence.ID != "S1151") || len(occurrence.Help) == 0 {
			t.Fatal(occurrence)
		}
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid task categories emitted artifact", err)
	}
}
