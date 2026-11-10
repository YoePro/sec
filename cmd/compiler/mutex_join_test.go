package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMutexJoinBoundaryCLI rejects live guards before potentially waiting joins and IR emission.
// Rules: rules/concurrency/mutex.md — §41(2–3).
func TestMutexJoinBoundaryCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/mutex_join_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/sema/mutex_join_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 6 {
			t.Fatal(output)
		}
		if !strings.Contains(output, "remains active across join") || !strings.Contains(output, "remains active across select") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid guard join emitted an artifact", err)
		}
	}
}
