package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCpuSetCLI checks canonical literals, diagnostics and the explicit native
// representation boundary without emitting a fabricated empty struct value.
// Rules: rules/concurrency/threads.md — §7; compiler/semantic_ir.md — unsupported lowering.
func TestCpuSetCLI(t *testing.T) {
	assertFormattingGoldenCLI(t, "cpu_set")
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/cpu_set_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/cpu_set_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 8 {
		t.Fatal(output)
	}
	if !strings.Contains(output, "S1214") || !strings.Contains(output, "duplicate CpuSet index") {
		t.Fatal(output)
	}
	artifact := filepath.Join(t.TempDir(), "cpuset.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/cpu_set_valid.sec", "-o", artifact)
	if code == 0 || !strings.Contains(output, "type CpuSet") || !strings.Contains(output, "unsupported-lowering") {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("unsupported CpuSet produced artifact", err)
	}
}
