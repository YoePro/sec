package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// The inspection boundaries use one track: schema MLIR keeps nominal enums,
// and the LLVM boundary discharges them after the enum representation stage.
// Rules: rules/compiler/compiler_pipeline.md — §§ 32–34, 59–60;
// rules/mlir/lowering-versions/sec_mlir_lowering_v7.md — sections 1–6.
func TestIntegerEnumTrackBoundaries(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	fixture := "../../testdata/build/native/integer_enums.sec"
	for _, command := range []string{"emit-sec-mlir", "emit-mlir", "emit-llvm"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, code := runCLIForDiagnostics(t, command, fixture, "-o", "-")
			if code != 0 {
				t.Fatalf("%s: %d %s", command, code, stderr)
			}
			if command == "emit-sec-mlir" {
				if !strings.Contains(stdout, "!sec.enum<") || !strings.Contains(stdout, "sec.enum.cmp") {
					t.Fatalf("nominal enum boundary lost:\n%s", stdout)
				}
			} else {
				for _, forbidden := range []string{"!sec.", "sec.enum.", "unrealized_conversion_cast"} {
					if strings.Contains(stdout, forbidden) {
						t.Errorf("%s retained %s", command, forbidden)
					}
				}
			}
		})
	}
}

// Representation support must not enable unchecked enum conversions or
// bit-backed hardware enums. A rejected build preserves existing output.
// Rules: rules/declarations/enums.md — §§ 10, 12;
// rules/compiler/compiler_pipeline.md — §§ 32–34, 59–60.
func TestIntegerEnumTrackRejectsDeferredOperations(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	for _, test := range []struct{ fixture, operation string }{
		{"unsupported_enum_conversion", "sec.enum.to_integer"},
		{"unsupported_bit_enum", "sec.enum.constant"},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "program")
			if err := os.WriteFile(output, []byte("previous output"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/"+test.fixture+".sec", "-o", output)
			if code != 4 || stdout != "" || !strings.Contains(stderr, "[unsupported-lowering]") || !strings.Contains(stderr, test.operation) {
				t.Fatalf("%d %s %s", code, stdout, stderr)
			}
			contents, err := os.ReadFile(output)
			if err != nil || string(contents) != "previous output" {
				t.Fatalf("rejected build changed output: %q %v", contents, err)
			}
		})
	}
}
