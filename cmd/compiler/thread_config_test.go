package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThreadConfigCLI uses the shared source contract and prevents invalid
// typed fields/ownership from publishing an artifact.
// Rules: rules/concurrency/threads.md — §§10–11.
func TestThreadConfigCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_config_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_config_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 || decodeOccurrenceDocument(t, output).Summary.Errors != 10 {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid configuration emitted IR", err)
	}
	// Source acceptance does not authorize native lowering before the separately
	// tracked thread creation/runtime implementation is available.
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_config_valid.sec", "-o", artifact)
	if code != 4 || !strings.Contains(output, "[unsupported-lowering]") || !strings.Contains(output, "ThreadConfig") {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("unsupported configuration emitted IR", err)
	}
}

// TestSchedulingThreadConfigurationCLI checks both rulebooks through the shared
// compiler frontend; invalid priority/affinity forms must not publish IR.
// Rules: rules/concurrency/scheduling.md — §§32–36;
// rules/concurrency/threads.md — §§4,6–7,10–11.
func TestSchedulingThreadConfigurationCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/scheduling_thread_config_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/scheduling_thread_config_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 || decodeOccurrenceDocument(t, output).Summary.Errors != 8 {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid scheduling configuration emitted IR", err)
	}
}
