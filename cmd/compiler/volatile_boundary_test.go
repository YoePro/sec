package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A valid volatile source reaches its explicit physical-plan gate rather than
// an ordinary ownership transfer or a partial MLIR output artifact.
// Rules: rules/memory/copy_move.md §19; rules/platform/volatile.md §§9–12;
// rules/compiler/compiler_pipeline.md §§32–34.
func TestVolatilePhysicalPlanBoundaryCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/semantic_ir/volatile_snapshot/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.sec")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", path)
	if code != 0 {
		t.Fatalf("source rejected: %d %s", code, stderr)
	}
	inspection := filepath.Join(root, "out.ir")
	_, stderr, code = runCLIForDiagnostics(t, "emit-ir", path, "-o", inspection)
	if code != 0 {
		t.Fatalf("IR inspection: %d %s", code, stderr)
	}
	ir, err := os.ReadFile(inspection)
	if err != nil || !strings.Contains(string(ir), "volatile.read") || !strings.Contains(string(ir), "physical-plan=unverified") {
		t.Fatalf("missing inspectable volatile effects: %v %s", err, ir)
	}
	artifact := filepath.Join(root, "out.mlir")
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", path, "-o", artifact)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "volatile.read needs a verified physical volatile access plan") {
		t.Fatalf("emission: %d %s %s", code, stdout, stderr)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("partial artifact: %v", err)
	}
}
