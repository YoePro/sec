package main

import (
	"os"
	"path/filepath"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// Source and inspection IR support the full open enum domain. Native bit-backed
// enum lowering remains a separately named unsupported implementation boundary.
// Rules: rules/declarations/enums.md — §§10,12.1;
// rules/compiler/semantic_ir.md — §124; compiler_pipeline.md — §§32–34.
func TestOpenEnumDomainCLI(t *testing.T) {
	const source = "../../testdata/semantic_ir/enum_open_domain.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "check", source)
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	artifact := filepath.Join(t.TempDir(), "open.ir")
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-ir", source, "-o", artifact)
	if code != 0 {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "enum.to-integer") != 3 || strings.Count(string(data), "proof=open-enum-domain") != 3 {
		t.Fatalf("missing complete-domain conversions:\n%s", data)
	}
	t.Run("native boundary", func(t *testing.T) {
		testsupport.RequireSecMLIROptPath(t)
		output := filepath.Join(t.TempDir(), "open")
		stdout, stderr, code := runCLIForDiagnostics(t, "build", source, "-o", output)
		if code == 0 || !strings.Contains(stderr, "[unsupported-lowering]") || !strings.Contains(stderr, "sec.enum.to_integer") {
			t.Fatal(code, stdout, stderr)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("partial native artifact: %v", err)
		}
	})
}
