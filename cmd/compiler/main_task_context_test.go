package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainTaskContextCLI checks both logical operations and all synchronous
// call forms before any Semantic IR artifact can be published.
// Rules: rules/concurrency/scheduling.md — §§21–22,50,57.
func TestMainTaskContextCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/main_task_context_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/main_task_context_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 || decodeOccurrenceDocument(t, output).Summary.Errors != 6 || strings.Count(output, `"id": "S1213"`) != 6 {
		t.Fatal(code, output)
	}
	for _, want := range []string{"Task.Current()", "Task.Yield()", "ordinary program entry is not a logical task", "current-execution requirement introduced here"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q: %s", want, output)
		}
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid main emitted IR", err)
	}
}
