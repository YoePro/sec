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

// RemoveAt of a trivially copyable or an owned element lowers to one guarded
// sec.dynamic_array.remove_at on the owner Place (v14 § 13C) that passes the real
// dialect and the P18 verification passes on 32- and 64-bit plans.
// Rules: rules/collections/collections.md — § 6.9;
// rules/mlir/dialect-versions/sec_mlir_dialect_v14.md — § 13C.
func TestDynamicArrayRemoveAtVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Fill() Result[uint, CollectionError] {\n    let mut values: byte[]\n    try values.Append(4)\n    let removed := values.RemoveAt(0)\n    discard removed\n    return Ok(values.Len)\n}\n\nfn main() int {\n    return 0\n}\n"
	nested := "module main\n\nfn F() void {\n    let mut values: int[][]\n    let removed := values.RemoveAt(0)\n    discard removed\n}\n\nfn main() int {\n    return 0\n}\n"
	for _, source := range []string{source, nested} {
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
				if strings.Count(string(output), "sec.dynamic_array.remove_at") != 1 {
					t.Fatalf("want one sec.dynamic_array.remove_at:\n%s", output)
				}
				verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places",
					"--sec-verify-ownership", "--sec-verify-destruction-plans", "-", "-o", os.DevNull)
				verify.Stdin = bytes.NewReader(output)
				if result, err := verify.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s\n%s", err, result, output)
				}
			})
		}
	}
}
