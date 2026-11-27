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

// Dynamic-array fill lowers to the schema-14 growth protocol in a loop and
// passes the real dialect and the P18 verification passes on 32- and 64-bit
// plans.
// Rules: rules/compiler/compiler_known_members.md — § 50(2).
func TestDynamicArrayFillVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Make(n: uint) Result[uint, CollectionError] {\n    let values: int[] := try fill(7, n)\n    return Ok(values.Len)\n}\n\nfn main() int {\n    return 0\n}\n"
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
			if !strings.Contains(string(output), "sec.dynamic_array.append_prepared") || !strings.Contains(string(output), "sec.dynamic_array.grow_backing") {
				t.Fatalf("fill lacks the growth protocol:\n%s", output)
			}
			verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places",
				"--sec-verify-ownership", "--sec-verify-destruction-plans", "--sec-verify-cleanups", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s\n%s", err, result, output)
			}
		})
	}
}
