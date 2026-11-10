package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaskYieldContextCLI preserves registered root-requirement diagnostics and
// their operation witnesses; invalid context cannot emit a Semantic IR artifact.
// Rules: rules/concurrency/scheduling.md — §§7(2),22,50,57;
// rules/concurrency/cancellation.md — §§19–23.
func TestTaskYieldContextCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/task_yield_context_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/sema/task_yield_context_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 7 {
			t.Fatal(output)
		}
		if strings.Count(output, `"id": "S1213"`) != 7 || !strings.Contains(output, "current-execution requirement introduced here") || !strings.Contains(output, "ordinary program entry is not a logical task") || !strings.Contains(output, "a spawned thread is a physical thread") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid context emitted IR", err)
		}
	}
}
