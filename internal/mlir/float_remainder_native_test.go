package mlir

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// Runtime binary significand reduction must agree bit-for-bit with an
// independent remainder implementation, including zero sign and extreme
// exponent differences; any NaN payload is permitted, but must stay NaN.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§23,75–80,86.
func TestFloatRemainderNativeDifferential(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	data, err := os.ReadFile("../../mlir/test/Conversion/SecToLLVM/float-remainder.mlir")
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	text.Write(data[:strings.LastIndex(string(data), "}")])
	text.WriteString(`func.func @main() -> i32 attributes {sec.function_id = "main::main()", sec.source_name = "main", sec.extern = false, sec.unsafe = false, sec.parameter_names = []} {
 %status0 = arith.constant 0 : i32
`)
	ordinal := 0
	random := rand.New(rand.NewSource(20))
	for _, width := range []int{32, 64} {
		tiny, largest := math.SmallestNonzeroFloat64, math.MaxFloat64
		normal := math.Float64frombits(0x0010000000000000)
		if width == 32 {
			tiny, largest = float64(math.SmallestNonzeroFloat32), float64(math.MaxFloat32)
			normal = float64(math.Float32frombits(0x00800000))
		}
		edges := []float64{0, math.Copysign(0, -1), 1, -1, 7.5, -2, tiny, -tiny, normal, normal - tiny, largest, math.Inf(1), math.Inf(-1), math.NaN()}
		pairs := [][2]float64{}
		for _, x := range edges {
			for _, y := range edges {
				pairs = append(pairs, [2]float64{x, y})
			}
		}
		for i := 0; i < 96; i++ {
			if width == 32 {
				pairs = append(pairs, [2]float64{float64(math.Float32frombits(random.Uint32())), float64(math.Float32frombits(random.Uint32()))})
			} else {
				pairs = append(pairs, [2]float64{math.Float64frombits(random.Uint64()), math.Float64frombits(random.Uint64())})
			}
		}
		for _, pair := range pairs {
			ordinal++
			for index, value := range pair {
				bits := math.Float64bits(value)
				if width == 32 {
					bits = uint64(math.Float32bits(float32(value)))
				}
				fmt.Fprintf(&text, "%%v%d_%d = \"sec.const.float\"() {lexeme = %q, bits = %d : i64} : () -> f%d\n", ordinal, index, strconv.FormatFloat(value, 'g', -1, width), int64(bits), width)
			}
			fmt.Fprintf(&text, "%%r%d = func.call @remainder%d(%%v%d_0, %%v%d_1) : (f%d, f%d) -> f%d\n", ordinal, width, ordinal, ordinal, width, width, width)
			expected := math.Mod(pair[0], pair[1])
			if width == 32 {
				expected = float64(float32(expected))
			}
			if math.IsNaN(expected) {
				fmt.Fprintf(&text, "%%ok%d = \"sec.float.classify\"(%%r%d) {kind = \"nan\"} : (f%d) -> i1\n", ordinal, ordinal, width)
			} else {
				bits := math.Float64bits(expected)
				if width == 32 {
					bits = uint64(math.Float32bits(float32(expected)))
				}
				fmt.Fprintf(&text, "%%b%d = arith.bitcast %%r%d : f%d to i%d\n", ordinal, ordinal, width, width)
				fmt.Fprintf(&text, "%%expected%d = arith.constant %d : i%d\n", ordinal, int64(bits), width)
				fmt.Fprintf(&text, "%%ok%d = arith.cmpi eq, %%b%d, %%expected%d : i%d\n", ordinal, ordinal, ordinal, width)
			}
			// All cases execute. Failure codes stay in 1..255 so a case index
			// divisible by 256 cannot accidentally look successful to the OS.
			fmt.Fprintf(&text, "%%code%d = arith.constant %d : i32\n%%status%d = arith.select %%ok%d, %%status%d, %%code%d : i32\n", ordinal, (ordinal-1)%255+1, ordinal, ordinal, ordinal-1, ordinal)
		}
	}
	fmt.Fprintf(&text, "return %%status%d : i32\n}\n}\n", ordinal)
	path := filepath.Join(t.TempDir(), "remainder-differential.mlir")
	if err := os.WriteFile(path, []byte(text.String()), 0600); err != nil {
		t.Fatal(err)
	}
	runFloatNativeFixture(t, path)
	t.Logf("executed %d edge/random cases over binary32 and binary64", ordinal)
}
