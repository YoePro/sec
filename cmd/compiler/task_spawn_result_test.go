package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaskSpawnResultCLI exercises creation handling at the compiler boundary
// and ensures invalid raw creation/owner confusion produces no IR artifact.
// Rules: rules/concurrency/tasks.md — §§4–5,30; rules/tooling/diagnostics.md — §14.
func TestTaskSpawnResultCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/task_spawn_result_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/task_spawn_result_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 5 || !strings.Contains(output, "Result[Task[int], TaskSpawnError]") || !strings.Contains(output, "await requires an owning Task[T]") {
		t.Fatal(output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid spawn emitted artifact", err)
	}
}
