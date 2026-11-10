package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThreadStorageCLI preserves ordinary ownership and compile-time validation,
// while rejecting native output until target-managed backing is implemented.
// Rules: rules/concurrency/threads.md — §8(1–7).
func TestThreadStorageCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_storage_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_storage_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 11 {
		t.Fatal(output)
	}
	for _, want := range []string{"S1215", "does not take type arguments", "read-only", "@noCopy"} {
		if !strings.Contains(output, want) {
			t.Fatal(want, output)
		}
	}
	artifact := filepath.Join(t.TempDir(), "storage.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_storage_valid.sec", "-o", artifact)
	if code == 0 || !strings.Contains(output, "unsupported-lowering") || !strings.Contains(output, "ThreadStorage") {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("unsupported storage emitted IR", err)
	}
}
