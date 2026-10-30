package semantic

import (
	"os"
	"strings"
	"testing"
)

// Decimal literal constants carry the signed int32 scale their spelling
// implies, as Sema resolved it: exponents give negative scales, written
// trailing zeros are kept, and a coefficient too wide for the target moves
// only to the closest exact coarser scale.
//
// Rules:
//   - rules/types/types.md — "Decimal scale"
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 2(2), 3(7), 14 DEC-02/DEC-03
func TestDecimalLiteralConstantsKeepSpelledScale(t *testing.T) {
	for _, tc := range []struct{ typ, literal, want string }{
		{"decimal", "1.20m", `coefficient=120 scale=2`},
		{"decimal", "1e3m", `coefficient=1 scale=-3`},
		{"decimal", "1.25e-3m", `coefficient=125 scale=5`},
		{"decimal", "0.000m", `coefficient=0 scale=3`},
		{"decimal", "10000000000000000000000.0m", `coefficient=1000000000000000000 scale=-4`},
		{"decimal128", "10000000000000000000000.0m", `coefficient=100000000000000000000000 scale=1`},
		{"decimal", "1e400m", `coefficient=1 scale=-400`},
		{"decimal", "1e400", `coefficient=1 scale=-400`},
		{"decimal128", "1.2500e400", `coefficient=12500 scale=-396`},
		{"decimal", "1e-400", `coefficient=1 scale=400`},
	} {
		source := "module main\nfn Read() " + tc.typ + " {\n    return " + tc.literal + "\n}\n"
		module, err := analyzedModule(t, source, 21)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.typ, tc.literal, err)
		}
		if printed := Format(module); !strings.Contains(printed, tc.want) {
			t.Errorf("%s %s: want %q in\n%s", tc.typ, tc.literal, tc.want, printed)
		}
	}
}

// Unsuffixed decimals beyond both ends of the host float64 range retain exact
// coefficients/scales through declared and inferred bindings into Semantic IR.
// Rules: rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §3(7).
func TestUnsuffixedDecimalBeyondBinaryRange(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/md066_literals/decimals.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 21)
	if err != nil {
		t.Fatal(err)
	}
	printed := Format(module)
	for text, count := range map[string]int{
		"coefficient=1 scale=-400":     2,
		"coefficient=12500 scale=-396": 1,
		"coefficient=1 scale=400":      1,
	} {
		if got := strings.Count(printed, text); got != count {
			t.Errorf("%q count %d, want %d in\n%s", text, got, count, printed)
		}
	}
}
