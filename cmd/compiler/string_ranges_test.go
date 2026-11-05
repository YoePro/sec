package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLI source checking accepts the MD-126 range/loop path and presents stable
// invalid-source/range errors. Native production retains its explicit P23 gate.
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
				if code == 0 || !strings.Contains(stderr, "StringView") || !strings.Contains(stderr, "P23") {
					t.Fatalf("missing named P23 gate: %d %s", code, stderr)
				}
				if _, err := os.Stat(artifact); !os.IsNotExist(err) {
					t.Fatalf("partial artifact: %v", err)
				}
			} else if code == 0 || !strings.Contains(stderr, "S1200") || !strings.Contains(stderr, "S1201") {
				t.Fatalf("missing range diagnostics: %d %s", code, stderr)
			}
		})
	}
}
