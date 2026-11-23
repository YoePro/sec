package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The public compiler accepts only the canonical attachment result surface;
// unsupported physical attachment cannot silently emit an executable IR body.
// Rules: rules/concurrency/thread_local.md §§30–33;
// rules/compiler/compiler_pipeline.md §32.
func TestThreadAttachmentCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_attachment/valid.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_attachment/calls_invalid.sec", "--diagnostic-format=json")
	if code == 0 || !strings.Contains(output, "ThreadAttachment") {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "attachment.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_attachment/valid.sec", "-o", artifact)
	if code == 0 || !strings.Contains(output, "unsupported-lowering") {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("unsupported attachment emitted IR", err)
	}

}
