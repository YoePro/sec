package layout

import (
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"
)

func sizeNumber(t *testing.T, text string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(text, 10)
	if !ok {
		t.Fatal(text)
	}
	return n
}

func sizeArithmetic(t *testing.T, maximum string) *SizeArithmetic {
	t.Helper()
	a, err := NewSizeArithmetic("selected-test-plan", sizeNumber(t, maximum), []*big.Int{big.NewInt(1), big.NewInt(3), big.NewInt(4), big.NewInt(8), big.NewInt(16)})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,15,48;
// rules/memory/layout.md — §§7–9,12,24.
func TestSizeArithmeticExactResults(t *testing.T) {
	a := sizeArithmetic(t, "18446744073709551615")
	for _, test := range []struct {
		name string
		call func() (*big.Int, error)
		want string
	}{
		{"add maximum", func() (*big.Int, error) {
			return a.CheckedAdd("field", sizeNumber(t, "18446744073709551600"), big.NewInt(15))
		}, "18446744073709551615"},
		{"multiply maximum", func() (*big.Int, error) {
			return a.CheckedMul("field", sizeNumber(t, "6148914691236517205"), big.NewInt(3))
		}, "18446744073709551615"},
		{"aligned", func() (*big.Int, error) { return a.AlignUp("field", big.NewInt(16), big.NewInt(8)) }, "16"},
		{"padding", func() (*big.Int, error) { return a.AlignUp("field", big.NewInt(17), big.NewInt(8)) }, "24"},
		{"supported nonbinary alignment", func() (*big.Int, error) { return a.AlignUp("field", big.NewInt(10), big.NewInt(3)) }, "12"},
		{"padded stride", func() (*big.Int, error) { return a.StrideOf("element", big.NewInt(10), big.NewInt(8), big.NewInt(0)) }, "16"},
		{"selected placement minimum", func() (*big.Int, error) { return a.StrideOf("element", big.NewInt(10), big.NewInt(8), big.NewInt(25)) }, "32"},
		{"zero stride policy", func() (*big.Int, error) { return a.StrideOf("empty", big.NewInt(0), big.NewInt(8), big.NewInt(0)) }, "0"},
		{"nonzero empty placement policy", func() (*big.Int, error) { return a.StrideOf("empty", big.NewInt(0), big.NewInt(8), big.NewInt(1)) }, "8"},
		{"array slots", func() (*big.Int, error) { return a.ArraySize("array", big.NewInt(16), big.NewInt(3)) }, "48"},
		{"empty array", func() (*big.Int, error) { return a.ArraySize("array", big.NewInt(16), big.NewInt(0)) }, "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.call()
			if err != nil || got == nil || got.String() != test.want {
				t.Fatalf("got %v, %v; want %s", got, err, test.want)
			}
		})
	}
}

// Target bounds and failure paths remain exact above uint64/uint256 and are
// independent of the compiler host. No failed computation publishes a value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,44,48.
func TestSizeArithmeticBoundsAndProvenance(t *testing.T) {
	for _, maximum := range []string{"4294967295", "18446744073709551615", "115792089237316195423570985008687907853269984665640564039457584007913129639935"} {
		a := sizeArithmetic(t, maximum)
		bound := sizeNumber(t, maximum)
		path := "Packet[uint256].payload[count=2]"
		for _, call := range []func() (*big.Int, error){
			func() (*big.Int, error) { return a.CheckedAdd(path, bound, big.NewInt(1)) },
			func() (*big.Int, error) { return a.CheckedMul(path, bound, big.NewInt(2)) },
			func() (*big.Int, error) { return a.AlignUp(path, bound, big.NewInt(8)) },
			func() (*big.Int, error) { return a.StrideOf(path, bound, big.NewInt(8), big.NewInt(0)) },
			func() (*big.Int, error) { return a.ArraySize(path, bound, big.NewInt(2)) },
		} {
			got, err := call()
			var failure *SizeArithmeticError
			if got != nil || !errors.As(err, &failure) || failure.Path != path || failure.Maximum != maximum || failure.Target != "selected-test-plan" {
				t.Fatalf("lost exact target/path: %v, %#v", got, err)
			}
			if sizeNumber(t, failure.Requested).Cmp(bound) <= 0 || !strings.Contains(err.Error(), failure.Requested) || !strings.Contains(err.Error(), maximum) || !strings.Contains(err.Error(), path) {
				t.Fatal("lost exact failing extent:", err)
			}
		}
	}
	a := sizeArithmetic(t, "4294967295")
	huge := new(big.Int).Lsh(big.NewInt(1), 300)
	count := new(big.Int).Set(huge)
	got, err := a.ArraySize("zero-sized[2^300]", big.NewInt(0), count)
	if err != nil || got.Sign() != 0 || count.Cmp(huge) != 0 {
		t.Fatalf("zero-sized logical count lost: %v %v %v", got, err, count)
	}
	if got, err := a.ArraySize("incomplete-element[0]", huge, big.NewInt(0)); err == nil || got != nil {
		t.Fatal("zero count hid an unrepresentable element stride")
	}
}

// Successful wide calculations must remain exact as well as wide failures.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§10,15,48.
func TestSizeArithmeticBeyondBackendIntegerWidths(t *testing.T) {
	maximum := new(big.Int).Lsh(big.NewInt(1), 512)
	a, err := NewSizeArithmetic("wide-address-space", maximum, []*big.Int{big.NewInt(8)})
	if err != nil {
		t.Fatal(err)
	}
	wide := new(big.Int).Lsh(big.NewInt(1), 300)
	got, err := a.CheckedAdd("wide.field", wide, wide)
	if err != nil || got.Cmp(new(big.Int).Lsh(big.NewInt(1), 301)) != 0 {
		t.Fatalf("wide addition: %v %v", got, err)
	}
	factor := new(big.Int).Lsh(big.NewInt(1), 150)
	got, err = a.CheckedMul("wide.field", factor, factor)
	if err != nil || got.Cmp(wide) != 0 {
		t.Fatalf("wide multiplication: %v %v", got, err)
	}
	got, err = a.ArraySize("wide.array", big.NewInt(16), wide)
	if err != nil || got.Cmp(new(big.Int).Lsh(big.NewInt(1), 304)) != 0 || wide.BitLen() != 301 {
		t.Fatalf("wide array extent/count: %v %v", got, err)
	}
}

// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§8–10,44;
// rules/memory/layout.md — §8(5).
func TestSizeArithmeticRejectsMissingFacts(t *testing.T) {
	for _, constructor := range []func() (*SizeArithmetic, error){
		func() (*SizeArithmetic, error) { return NewSizeArithmetic("", big.NewInt(1), nil) },
		func() (*SizeArithmetic, error) { return NewSizeArithmetic("target", nil, nil) },
		func() (*SizeArithmetic, error) { return NewSizeArithmetic("target", big.NewInt(-1), nil) },
		func() (*SizeArithmetic, error) { return NewSizeArithmetic("target", big.NewInt(1), []*big.Int{nil}) },
		func() (*SizeArithmetic, error) {
			return NewSizeArithmetic("target", big.NewInt(1), []*big.Int{big.NewInt(0)})
		},
		func() (*SizeArithmetic, error) {
			return NewSizeArithmetic("target", big.NewInt(1), []*big.Int{big.NewInt(-1)})
		},
	} {
		if a, err := constructor(); err == nil || a != nil {
			t.Fatal("accepted unresolved capabilities")
		}
	}
	a := sizeArithmetic(t, "100")
	for _, call := range []func() (*big.Int, error){
		func() (*big.Int, error) { return a.CheckedAdd("", big.NewInt(0), big.NewInt(0)) },
		func() (*big.Int, error) { return a.CheckedAdd("field", nil, big.NewInt(1)) },
		func() (*big.Int, error) { return a.CheckedMul("field", big.NewInt(-1), big.NewInt(0)) },
		func() (*big.Int, error) { return a.ArraySize("field", big.NewInt(1), big.NewInt(-1)) },
		func() (*big.Int, error) { return a.AlignUp("field", big.NewInt(1), big.NewInt(0)) },
		func() (*big.Int, error) { return a.AlignUp("field", big.NewInt(1), big.NewInt(2)) },
		func() (*big.Int, error) { return a.StrideOf("field", big.NewInt(1), big.NewInt(1), nil) },
		func() (*big.Int, error) { return new(SizeArithmetic).CheckedAdd("field", big.NewInt(0), big.NewInt(0)) },
		func() (*big.Int, error) {
			return (*SizeArithmetic)(nil).CheckedAdd("field", big.NewInt(0), big.NewInt(0))
		},
	} {
		if got, err := call(); err == nil || got != nil {
			t.Fatal("accepted unresolved, negative or unsupported operand")
		}
	}
}

// Shared arithmetic snapshots never alias caller inputs/results, including
// concurrent readers and targets with different bounds/alignment capabilities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package24.md — §§8–10,51.
func TestSizeArithmeticSnapshotIsolation(t *testing.T) {
	maximum, alignment := big.NewInt(100), big.NewInt(8)
	a, err := NewSizeArithmetic("first", maximum, []*big.Int{alignment})
	if err != nil {
		t.Fatal(err)
	}
	maximum.SetInt64(1)
	alignment.SetInt64(3)
	input := big.NewInt(9)
	got, err := a.AlignUp("field", input, big.NewInt(8))
	if err != nil || got.Int64() != 16 || input.Int64() != 9 {
		t.Fatalf("aliased input capabilities: %v %v", got, err)
	}
	got.SetInt64(99)
	other, _ := NewSizeArithmetic("second", big.NewInt(12), []*big.Int{big.NewInt(3)})
	if value, err := other.AlignUp("field", input, big.NewInt(8)); err == nil || value != nil {
		t.Fatal("another target borrowed alignment support")
	}
	var readers sync.WaitGroup
	for i := 0; i < 32; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			value, err := a.AlignUp("field", input, big.NewInt(8))
			if err != nil || value.Int64() != 16 {
				t.Errorf("snapshot changed: %v %v", value, err)
			}
		}()
	}
	readers.Wait()
}
