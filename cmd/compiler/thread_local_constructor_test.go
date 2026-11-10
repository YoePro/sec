package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThreadLocalConstructorCLI checks constructor eligibility before IR emission.
// Rules: rules/concurrency/thread_local.md — §7(1–6).
func TestThreadLocalConstructorCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_local_constructor_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/sema/thread_local_constructor_invalid.sec", "--diagnostic-format=json"}
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
		if !strings.Contains(output, "S1209") || !strings.Contains(output, "thread-local.initial-not-copyable") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid TLS key emitted an artifact", err)
		}
	}
}
