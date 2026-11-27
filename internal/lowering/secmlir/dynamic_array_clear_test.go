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

// Clear lowers to one
// sec.dynamic_array.clear on the owner Place (v14 § 13B), with the element
// destruction plan when the element needs destruction, that passes the real
// dialect and the P18 verification passes on 32- and 64-bit plans.
// Rules: rules/collections/collections.md — § 6.8;
// rules/mlir/dialect-versions/sec_mlir_dialect_v14.md — § 13B.
func TestDynamicArrayClearVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Fill() Result[uint, CollectionError] {\n    let mut values: byte[]\n    try values.Append(4)\n    values.Clear()\n    return Ok(values.Len)\n}\n\nfn main() int {\n    return 0\n}\n"
	nested := "module main\n\nfn F() void {\n    let mut values: int[][]\n    values.Clear()\n}\n\nfn main() int {\n    return 0\n}\n"
	for _, item := range []struct{ source, want string }{
		{source, `"sec.dynamic_array.clear"(`},
		{nested, `destruction_plan = "destroy:array:dynamic::int:int"`},
	} {
		source := item.source
		want := item.want
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
				if strings.Count(string(output), "sec.dynamic_array.clear") != 1 || !strings.Contains(string(output), want) {
					t.Fatalf("want one sec.dynamic_array.clear:\n%s", output)
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
