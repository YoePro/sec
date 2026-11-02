package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// Source statics lower through the ordinary global representation in the one
// compilation track, without section selection, fixed-address pointer casts,
// MMIO/volatile operations or non-default LLVM address spaces.
// Rules: rules/declarations/static.md — §22;
// rules/foundations/attributes.md — physical placement boundary;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestStaticPhysicalPlacementNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	file := "../../testdata/semantic_ir/static_ordinary_space.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file, "-o", "-")
	if code != 0 || strings.Count(stdout, `"sec.static.global"`) != 4 {
		t.Fatal(code, stdout, stderr)
	}
	for _, bad := range []string{"section =", "address_space =", "mmio", "volatile", "inttoptr"} {
		if strings.Contains(stdout, bad) {
			t.Fatalf("static selected physical placement %q:\n%s", bad, stdout)
		}
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-llvm", file, "-o", "-")
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	for _, want := range []string{"internal constant i64 7", "internal constant i64 8", "internal global i64 0"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("lost ordinary global %q:\n%s", want, stdout)
		}
	}
	for _, bad := range []string{`section "`, "addrspace(", "inttoptr", "volatile"} {
		if strings.Contains(stdout, bad) {
			t.Fatalf("static selected physical placement %q:\n%s", bad, stdout)
		}
	}
}

// static cannot authorize a placement attribute outside the closed set. The
// public command must reject the source before producing a lowering artifact.
// Rules: rules/declarations/static.md — §22; rules/tooling/diagnostics.md — mandatory errors.
func TestStaticPhysicalPlacementRejectionsCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/sema/static_placement/placement_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.sec")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "out.ir")
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact, "--diagnostic-format=json")
	if code == 0 || stdout != "" {
		t.Fatal(code, stdout, stderr)
	}
	document := decodeOccurrenceDocument(t, stderr)
	if document.Summary.Errors != 2 || len(document.Occurrences) != 2 {
		t.Fatal(stderr)
	}
	for _, d := range document.Occurrences {
		if d.ID == nil || *d.ID != diagnostics.UnknownAttribute || d.Unregistered {
			t.Fatal(d)
		}
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("invalid placement produced an artifact: %v", err)
	}
}
