package semantic

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

// MD-115 §§4–7,10: source-owned exact/dynamic ranges survive normalization,
// copies, calls and full-open reborrows on both target plans.
func TestPackage16RangeReturns(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_range_returns.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]string{"CallStart": "[2..<3]", "EndOnly": "[0..<3]", "NestedStart": "[12..<20]", "Fixed": "[1..<3]", "Exact": "[1..<3]", "Empty": "[2..<2]", "Mutable": "[1..<3]", "Reduced": "[1..<3]", "Dynamic": "[?]", "StartOnly": "[?]", "Full": "[1..<3]", "Nested": "[12..<15]", "Called": "[11..<13]", "Inclusive": "[1..<3]", "Copied": "[1..<3]"} {
			fn := functionNamed(t, module, name)
			got := formatReturnOrigins(fn.ReturnOrigins)
			if got != " returns-from=[param#0"+want+"]" {
				t.Fatalf("%s: %s", name, got)
			}
		}
		field := functionNamed(t, module, "Field")
		if len(field.ReturnOrigins) != 1 || len(field.ReturnOrigins[0].Path) != 2 || field.ReturnOrigins[0].Path[0].Kind != ReturnOriginField || !strings.Contains(formatReturnOrigins(field.ReturnOrigins), "[1..<3]") {
			t.Fatal("lost field/range path")
		}
		alternatives := functionNamed(t, module, "Alternatives")
		if got := formatReturnOrigins(alternatives.ReturnOrigins); got != " returns-from=[param#0[1..<3], param#0[4..<6]]" {
			t.Fatalf("lost alternatives: %s", got)
		}
		again, err := buildPackage16Source(t, string(source), width)
		if err != nil || Format(again) != Format(module) {
			t.Fatalf("unstable build: %v", err)
		}
		exact := functionNamed(t, module, "Exact")
		exact.ReturnOrigins[0].Path[0].EndExclusive = big.NewInt(4)
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "does not list") {
			t.Fatalf("forged exact range: %v", err)
		}
	}
}

// MD-115 §§4(2),5(3): arbitrary-precision path composition detaches integers.
func TestReturnOriginRangePrecision(t *testing.T) {
	start, _ := new(big.Int).SetString("184467440737095516160", 10)
	end := new(big.Int).Add(start, big.NewInt(20))
	p := []ReturnOriginStep{{Kind: ReturnOriginSlice, Start: start, EndExclusive: end}}
	result := appendReturnOriginPath(p, []ReturnOriginStep{{Kind: ReturnOriginSlice, Start: big.NewInt(2), EndExclusive: big.NewInt(5)}})
	if result[0].Start.String() != "184467440737095516162" || result[0].EndExclusive.String() != "184467440737095516165" {
		t.Fatal(result)
	}
	result[0].Start.SetInt64(0)
	if start.String() != "184467440737095516160" {
		t.Fatal("aliased range")
	}
}
