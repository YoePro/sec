package secmlir

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// MD-087: a module mixing reference-free scalar functions with functions
// that compute through `ref mut` lowers through P5/P6/P8 on 32- and 64-bit
// plans. The scalar function becomes signless Arith; checked arithmetic on a
// referent reads the semantic siN/uiN Place value, bridges it to the signless
// computation and back with exact-width casts only, keeps signed/unsigned
// interpretation, and writes it through the unchanged Place; address-taken
// storage stays `sec.storage` and reference-bearing calls stay
// `sec.call.direct`. The result passes every Place/reference verifier.
//
// Rules:
//   - rules/mlir/sec_mlir_dialect.md — v11.31–v11.33 amendments (MD-087)
//   - rules/mlir/sec_mlir_lowering.md — v11.18–v11.20 amendments
func TestCheckedIntegerLoweringCoexistsWithReferences(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn Scalar(a: int) int {
    return a * 2
}

fn Increment(value: ref mut int) void {
    *value += 1
}

fn Halve(value: ref mut uint32) void {
    *value = *value / 2
}

fn main() int {
    let mut n := 41
    Increment(n)
    let mut u: uint32 := 10
    Halve(u)
    return Scalar(n) - 84
}
`
	bridge := regexp.MustCompile(`builtin\.unrealized_conversion_cast %\S+ : (s|u)?i(\d+) to (s|u)?i(\d+)`)
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(buildPackage15EmitModule(t, "mixed", source), testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "mixed.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(tool, path, "--sec-lower-integer-core", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics")
			lowered, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("mixed module did not lower: %v\n%s", err, lowered)
			}
			text := string(lowered)
			bits := strconv.Itoa(int(width))
			for _, want := range []string{
				"(%arg0: i" + bits + " {sec.scalar_kind = \"int\"}) -> (i" + bits,
				`"sec.place.read"(`, `"sec.place.write"(`,
				"!sec.storage<si" + bits + ">", "sec.address_taken = true",
				`"sec.call.direct"(`, `sec.argument_actions = ["mutable-reference"]`,
				"arith.divui",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in:\n%s", want, text)
				}
			}
			for _, forbidden := range []string{"sec.int.binary_checked", "arith.bitcast", "memref.alloca"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("unexpected %q in:\n%s", forbidden, text)
				}
			}
			matches := bridge.FindAllStringSubmatch(text, -1)
			if len(matches) == 0 {
				t.Fatalf("no Place boundary bridge in:\n%s", text)
			}
			for _, match := range matches {
				if match[2] != match[4] || (match[1] == "") == (match[3] == "") {
					t.Errorf("bridge is not an exact-width semantic/signless transition: %s", match[0])
				}
			}
		})
	}
}
