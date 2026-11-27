package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// Contextual fixed-array fill(value) emits one sec.array.fill per call that
// passes the real dialect and the ownership verification on 32- and 64-bit
// plans.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/mlir/dialect-versions/sec_mlir_dialect_v10.md — § 10A.
func TestFixedArrayFillVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/ir/test_fixed_array_fill.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			arch := "amd64"
			if width == 32 {
				arch = "armv7"
			}
			module, err := buildPackage18Source(t, string(source), arch, width)
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(output), `"sec.array.fill"`) != 2 || !strings.Contains(string(output), `!sec.array<!sec.int, "4">`) {
				t.Fatalf("want two sec.array.fill:\n%s", output)
			}
			verify := exec.Command(tool, "--sec-verify-ownership", "--sec-verify-destruction-plans", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s\n%s", err, result, output)
			}
		})
	}
}
