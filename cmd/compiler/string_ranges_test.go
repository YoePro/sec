package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI source checking accepts the MD-126 range/loop path and presents stable
// invalid-source/range errors. Semantic IR and verified Sec MLIR represent the
// views; native production retains its explicit P23 gate.
// Rules: rules/collections/collections.md — §7.10; diagnostics.md — §5;
// rules/compiler/semantic_ir.md — §17(9–10).
func TestStringRangesCompilerCLI(t *testing.T) {
	for _, name := range []string{"valid", "range_invalid"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0700); err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile("../../testdata/sema/string_ranges/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "main.sec")
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			_, stderr, code := runCLIForDiagnostics(t, "check", path)
			if name == "valid" {
				if code != 0 {
					t.Fatalf("valid source rejected: %d %s", code, stderr)
				}
				artifact := filepath.Join(root, "out.ir")
				_, stderr, code = runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact)
				if code != 0 {
					t.Fatalf("Semantic IR rejected valid string ranges: %d %s", code, stderr)
				}
				if data, err := os.ReadFile(artifact); err != nil || !strings.Contains(string(data), "string.view-borrow") || !strings.Contains(string(data), "string-view.decode-next") {
					t.Fatalf("Semantic IR lacks string view operations: %v", err)
				}
				native := filepath.Join(root, "out.ll")
				_, stderr, code = runCLIForDiagnostics(t, "emit-llvm", path, "-o", native)
				// The native track stops at its first unsupported physical
				// representation (references before P23 strings/views).
				if code == 0 || !strings.Contains(stderr, "unsupported-lowering") {
					t.Fatalf("missing native lowering gate: %d %s", code, stderr)
				}
				if _, err := os.Stat(native); !os.IsNotExist(err) {
					t.Fatalf("partial artifact: %v", err)
				}
			} else if code == 0 || !strings.Contains(stderr, "S1200") || !strings.Contains(stderr, "S1201") {
				t.Fatalf("missing range diagnostics: %d %s", code, stderr)
			}
		})
	}
}

// Checking follows the complete MD-126 try frontend path, including recovery
// provenance and unrelated panic diagnostics, before any native lowering gate.
// Rules: rules/errors/errorhandling.md — §§9,21–22,31;
// rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md — §§3–4.
func TestStringRangeTryCompilerCLI(t *testing.T) {
	for _, tc := range []struct {
		name  string
		codes []string
	}{
		{"try_valid", nil},
		{"try_origin_invalid", []string{"S1128", "S1162"}},
		{"try_invalid", []string{"S1092", "S1093", "S1201"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0700); err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile("../../testdata/sema/string_ranges/" + tc.name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "main.sec")
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			_, stderr, code := runCLIForDiagnostics(t, "check", path)
			if (code == 0) != (len(tc.codes) == 0) {
				t.Fatalf("unexpected check result: %d %s", code, stderr)
			}
			for _, id := range tc.codes {
				if !strings.Contains(stderr, id) {
					t.Fatalf("missing %s: %s", id, stderr)
				}
			}
		})
	}
}
