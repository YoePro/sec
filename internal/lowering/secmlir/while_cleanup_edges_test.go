package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// While loops whose bodies own non-trivially destructible storage or a defer
// carry explicit cleanup edges, loop-carried ownership stays explicit, and
// while loops nest in for and switch (Semantic IR § 66). Each program
// verifies in Semantic IR, shows its cleanup forms, and passes the real
// dialect's cleanup, ownership and destruction-plan verification.
// Rules: rules/compiler/semantic_ir.md — § 66(1)–(3);
// rules/control-flow/flowcontrol_while.md — § 29.
func TestWhileLoopCleanupEdgesEndToEnd(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	tail := "\nfn main() int {\n    return 0\n}\n"
	for _, item := range []struct {
		name, source string
		want         []string
	}{
		{"owned-body", "module main\n\nfn F(n: uint) Result[uint, CollectionError] {\n    let mut i: uint := 0\n    while i < n {\n        let mut values: int[]\n        try values.Append(1)\n        i = i + 1\n    }\n    return Ok(i)\n}\n",
			[]string{"cleanup.track-owned", "exit=error-propagation", "exit=fallthrough", "edge=back"}},
		{"defer-break", "module main\n\nfn G() void {\n}\n\nfn F(n: uint) void {\n    let mut i: uint := 0\n    while i < n {\n        defer {\n            G()\n        }\n        if i == 3 {\n            break\n        }\n        i = i + 1\n    }\n}\n",
			[]string{"cleanup.defer-register", "edge=break", "edge=back"}},
		{"loop-carried-move", "module main\n\nfn Consume(values: int[]) void {\n}\n\nfn F(n: uint) Result[uint, CollectionError] {\n    let mut values: int[]\n    try values.Append(1)\n    let mut i: uint := 0\n    while i < n {\n        if i == 2 {\n            Consume(<-values)\n            break\n        }\n        i = i + 1\n    }\n    return Ok(i)\n}\n",
			[]string{"cleanup.cancel", "reason=transferred", "edge=break"}},
		{"for-nesting", "module main\n\nfn F(n: int) void {\n    for i in 0..n {\n        let mut j: int := 0\n        while j < i {\n            j = j + 1\n        }\n    }\n}\n",
			[]string{"loop=@1", "loop=@2"}},
		{"switch-nesting", "module main\n\nfn F(n: int) int {\n    let mut total: int := 0\n    switch n {\n    case 1:\n        let mut j: int := 0\n        while j < 3 {\n            j = j + 1\n        }\n        total = j\n    default:\n        total = 0\n    }\n    return total\n}\n",
			[]string{"edge=back"}},
	} {
		t.Run(item.name, func(t *testing.T) {
			for _, width := range []uint16{32, 64} {
				arch := "amd64"
				if width == 32 {
					arch = "armv7"
				}
				module, err := buildPackage18Source(t, item.source+tail, arch, width)
				if err != nil {
					t.Fatal(err)
				}
				if err := semantic.Verify(module); err != nil {
					t.Fatal(err)
				}
				text := semantic.Format(module)
				for _, want := range item.want {
					if !strings.Contains(text, want) {
						t.Fatalf("%d-bit: missing %q in:\n%s", width, want, text)
					}
				}
				output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 14})
				if err != nil {
					t.Fatal(err)
				}
				verify := exec.Command(tool, "--sec-verify-cleanups", "--sec-verify-ownership", "--sec-verify-destruction-plans",
					"--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places", "-", "-o", os.DevNull)
				verify.Stdin = bytes.NewReader(output)
				if result, err := verify.CombinedOutput(); err != nil {
					t.Fatalf("%d-bit: %v\n%s\n%s", width, err, result, fmt.Sprint(string(output)))
				}
			}
		})
	}
}
