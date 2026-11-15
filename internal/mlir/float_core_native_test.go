package mlir

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// Core float operations must agree with an independent numerical oracle at
// binary boundaries, arbitrary bit patterns and signed ties. Operands cross
// function boundaries; results are compared by bits except for unspecified NaN
// payloads. The native pipeline links no math library. Clamp has its own
// candidate/failure matrix in TestFloatClampNativeMatrix.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§28–36,78,80,87.
func TestFloatCoreNativeDifferential(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	for _, width := range []int{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var text strings.Builder
			text.WriteString(`module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["float-core-matrix.mlir"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
`)
			type unary struct {
				op, kind string
				oracle   func(float64) float64
			}
			unaryOps := []unary{{"abs", "", math.Abs}, {"round", "floor", math.Floor}, {"round", "ceiling", math.Ceil}, {"round", "truncate", math.Trunc}, {"round", "nearest-even", math.RoundToEven}}
			classify := []string{"finite", "infinite", "nan", "normal", "subnormal"}
			for _, u := range unaryOps {
				name := u.op + strings.ReplaceAll(u.kind, "-", "_")
				attr := ""
				if u.kind != "" {
					attr = fmt.Sprintf(" {kind = %q}", u.kind)
				}
				fmt.Fprintf(&text, "func.func @%s(%%x: f%d) -> f%d { %%r = \"sec.float.%s\"(%%x)%s : (f%d) -> f%d\n return %%r : f%d\n}\n", name, width, width, u.op, attr, width, width, width)
			}
			for _, kind := range classify {
				fmt.Fprintf(&text, "func.func @%s(%%x: f%d) -> i1 { %%r = \"sec.float.classify\"(%%x) {kind = %q} : (f%d) -> i1\n return %%r : i1\n}\n", kind, width, kind, width)
			}
			for _, kind := range []string{"minimum", "maximum"} {
				fmt.Fprintf(&text, "func.func @%s(%%x: f%d, %%y: f%d) -> f%d { %%r = \"sec.float.%s\"(%%x, %%y) : (f%d, f%d) -> f%d\n return %%r : f%d\n}\n", kind, width, width, width, kind, width, width, width, width)
			}
			text.WriteString("func.func @main() -> i32 {\n %status0 = arith.constant 0 : i32\n")
			ordinal := 0
			bitsOf := func(x float64) uint64 {
				if width == 32 {
					return uint64(math.Float32bits(float32(x)))
				}
				return math.Float64bits(x)
			}
			valueOf := func(bits uint64) float64 {
				if width == 32 {
					return float64(math.Float32frombits(uint32(bits)))
				}
				return math.Float64frombits(bits)
			}
			// Explicit raw NaN payloads include quiet/signaling, both signs and nonzero
			// low payload bits, rather than relying only on one host NaN constructor.
			edges := []uint64{0, 0x8000000000000000, 1, 0x000fffffffffffff, 0x0010000000000000, 0x7fefffffffffffff, 0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000000, 0x7ff0000000000001, 0xfff8123456789abc}
			if width == 32 {
				edges = []uint64{0, 0x80000000, 1, 0x007fffff, 0x00800000, 0x7f7fffff, 0x7f800000, 0xff800000, 0x7fc00000, 0x7f800001, 0xffc12345}
			}
			signed := append([]uint64{}, edges...)
			for _, bits := range edges[2:6] {
				signed = append(signed, bits|uint64(1)<<(width-1))
			}
			for _, x := range []float64{0.25, 0.5, 0.75, 1.5, 2.5, 3.5, 2.7, math.Ldexp(1, 63)} {
				signed = append(signed, bitsOf(x), bitsOf(-x))
			}
			precision := 52
			if width == 32 {
				precision = 23
			}
			cutoff := math.Ldexp(1, precision)
			for _, x := range []float64{cutoff - 0.5, cutoff, cutoff + 1} {
				signed = append(signed, bitsOf(x), bitsOf(-x))
			}
			// Every deterministic edge, in both operand orders, also participates in
			// Min/Max. Random unary cases below broaden classification and rounding.
			binaryEdges := append([]uint64{}, signed...)
			random := rand.New(rand.NewSource(2031))
			for n := 0; n < 32; n++ {
				bits := random.Uint64()
				if width == 32 {
					bits = uint64(uint32(bits))
				}
				signed = append(signed, bits)
			}
			emitConst := func(name string, bits uint64) {
				fmt.Fprintf(&text, "%%%s = arith.constant %d : i%d\n%%%sf = arith.bitcast %%%s : i%d to f%d\n", name, int64(bits), width, name, name, width, width)
			}
			finish := func() {
				fmt.Fprintf(&text, "%%code%d = arith.constant %d : i32\n%%status%d = arith.select %%ok%d, %%status%d, %%code%d : i32\n", ordinal, (ordinal-1)%255+1, ordinal, ordinal, ordinal-1, ordinal)
			}
			checkFloat := func(expected float64) {
				if math.IsNaN(expected) {
					fmt.Fprintf(&text, "%%ok%d = arith.cmpf uno, %%r%d, %%r%d : f%d\n", ordinal, ordinal, ordinal, width)
				} else {
					fmt.Fprintf(&text, "%%b%d = arith.bitcast %%r%d : f%d to i%d\n%%expected%d = arith.constant %d : i%d\n%%ok%d = arith.cmpi eq, %%b%d, %%expected%d : i%d\n", ordinal, ordinal, width, width, ordinal, int64(bitsOf(expected)), width, ordinal, ordinal, ordinal, width)
				}
				finish()
			}
			normal := math.Float64frombits(0x0010000000000000)
			if width == 32 {
				normal = float64(math.Float32frombits(0x00800000))
			}
			for _, bits := range signed {
				x := valueOf(bits)
				for _, u := range unaryOps {
					ordinal++
					emitConst(fmt.Sprintf("x%d", ordinal), bits)
					fmt.Fprintf(&text, "%%r%d = func.call @%s(%%x%df) : (f%d) -> f%d\n", ordinal, u.op+strings.ReplaceAll(u.kind, "-", "_"), ordinal, width, width)
					checkFloat(u.oracle(x))
				}
				finite := !math.IsNaN(x) && !math.IsInf(x, 0)
				expected := []bool{finite, math.IsInf(x, 0), math.IsNaN(x), finite && math.Abs(x) >= normal, finite && x != 0 && math.Abs(x) < normal}
				for i, kind := range classify {
					ordinal++
					emitConst(fmt.Sprintf("x%d", ordinal), bits)
					fmt.Fprintf(&text, "%%r%d = func.call @%s(%%x%df) : (f%d) -> i1\n%%expected%d = arith.constant %t\n%%ok%d = arith.cmpi eq, %%r%d, %%expected%d : i1\n", ordinal, kind, ordinal, width, ordinal, expected[i], ordinal, ordinal, ordinal)
					finish()
				}
			}
			for _, left := range binaryEdges {
				for _, right := range binaryEdges {
					for _, kind := range []string{"minimum", "maximum"} {
						ordinal++
						emitConst(fmt.Sprintf("x%d", ordinal), left)
						emitConst(fmt.Sprintf("y%d", ordinal), right)
						fmt.Fprintf(&text, "%%r%d = func.call @%s(%%x%df, %%y%df) : (f%d, f%d) -> f%d\n", ordinal, kind, ordinal, ordinal, width, width, width)
						x, y := valueOf(left), valueOf(right)
						expected := math.Min(x, y)
						if kind == "maximum" {
							expected = math.Max(x, y)
						}
						// Go's Min/Max prioritize some infinities over NaN; Sec always propagates NaN.
						if math.IsNaN(x) || math.IsNaN(y) {
							expected = math.NaN()
						}
						checkFloat(expected)
					}
				}
			}
			largest, epsilon := math.MaxFloat64, math.Ldexp(1, -52)
			if width == 32 {
				largest, epsilon = math.MaxFloat32, math.Ldexp(1, -23)
			}
			for _, p := range []struct {
				kind  string
				value float64
			}{{"min", -largest}, {"max", largest}, {"epsilon", epsilon}, {"infinity", math.Inf(1)}, {"negative-infinity", math.Inf(-1)}, {"nan", math.NaN()}} {
				ordinal++
				fmt.Fprintf(&text, "%%r%d = \"sec.float.property\"() {kind = %q} : () -> f%d\n", ordinal, p.kind, width)
				checkFloat(p.value)
			}
			fmt.Fprintf(&text, "return %%status%d : i32\n}\n}\n", ordinal)
			path := filepath.Join(t.TempDir(), "core-differential.mlir")
			if err := os.WriteFile(path, []byte(text.String()), 0600); err != nil {
				t.Fatal(err)
			}
			runFloatNativeFixture(t, path)
			t.Logf("executed %d bit-exact/classification checks over binary%d", ordinal, width)
		})
	}
}
