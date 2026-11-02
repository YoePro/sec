package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Valid fallible enum conversions stop at the named checked-lowering boundary
// without an IR artifact. Sema acceptance and exact IR emission are distinct.
// Rules: rules/declarations/enums.md — §12.1;
// rules/compiler/compiler_pipeline.md — §§32–34; rules/compiler/semantic_ir.md — §124.
func TestEnumCheckedConversionEmissionBoundaryCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fallible", "runtime_member", "exact"} {
		data, err := os.ReadFile("../../testdata/semantic_ir/enum_checked_boundary/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runCLIForDiagnostics(t, "check", path)
		if code != 0 {
			t.Fatal(code, stderr)
		}
		artifact := filepath.Join(root, name+".ir")
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact)
		if name != "exact" {
			if code == 0 || stdout != "" || !strings.Contains(stderr, "[unsupported-lowering]") || !strings.Contains(stderr, "fallible enum to integer conversion Wide to uint8 (needs checked failure control flow)") {
				t.Fatal(code, stdout, stderr)
			}
			if _, err := os.Stat(artifact); !os.IsNotExist(err) {
				t.Fatalf("partial output artifact: %v", err)
			}
		} else {
			if code != 0 {
				t.Fatal(code, stdout, stderr)
			}
			output, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(output), "enum.to-integer") != 2 {
				t.Fatalf("missing proven exact conversions:\n%s", output)
			}
		}
	}
}
