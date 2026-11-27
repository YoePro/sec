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

// Mutable-slice Fill emits one sec.slice.fill (v12 § 25A) that passes the
// real dialect, the slice-guard, place, reference-guard and borrow
// verification on 32- and 64-bit plans.
// Rules: rules/collections/collections.md — § 7.9;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — § 25A.
func TestSliceFillVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Reset(values: ref mut int[]) void {\n    values.Fill(7)\n}\n\nfn main() int {\n    let mut data: int[3] := [1, 2, 3]\n    Reset(ref mut data[..])\n    return data[0]\n}\n"
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			arch := "amd64"
			if width == 32 {
				arch = "armv7"
			}
			module, err := buildPackage18Source(t, source, arch, width)
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(output), `"sec.slice.fill"`) != 1 {
				t.Fatalf("want one sec.slice.fill:\n%s", output)
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards",
				"--sec-verify-borrow-semantics", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s\n%s", err, result, output)
			}
		})
	}
}
