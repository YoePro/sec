package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThreadSpawnResultCLI checks raw creation handling and rejects invalid
// owner confusion before IR emission; no native thread runtime is implied.
// Rules: rules/concurrency/threads.md — §§2,19,34; diagnostics.md — §14.
func TestThreadSpawnResultCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_spawn_result_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_spawn_result_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 7 || !strings.Contains(output, "Result[Thread[int], ThreadSpawnError]") || !strings.Contains(output, "join requires Task[T] or Thread[T]") {
		t.Fatal(output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid thread creation emitted artifact", err)
	}
}

// TestThreadSpawnResultLoweringBoundary keeps correct source typing separate
// from unavailable thread representation and preserves existing output.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/concurrency/threads.md — §§2,19.
func TestThreadSpawnResultLoweringBoundary(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "existing.ir")
	previous := []byte("existing output")
	if err := os.WriteFile(artifact, previous, 0600); err != nil {
		t.Fatal(err)
	}
	_, output, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_spawn_result_valid.sec", "-o", artifact)
	if code != 4 || !strings.Contains(output, "[unsupported-lowering]") || !strings.Contains(output, "type Thread") {
		t.Fatal(code, output)
	}
	current, err := os.ReadFile(artifact)
	if err != nil || string(current) != string(previous) {
		t.Fatal("failed lowering changed output", string(current), err)
	}
}
