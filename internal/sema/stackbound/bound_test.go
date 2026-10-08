package stackbound

import (
	"math/big"
	"testing"
)

// TestStackBoundClassifications keeps all four proof qualities distinct and
// ensures a missing bound cannot be consumed as a zero-byte resource proof.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification" and "Unknown".
func TestStackBoundClassifications(t *testing.T) {
	exact, err := NewExact(big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	upper, err := NewUpper(big.NewInt(8192))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		bound  Bound
		kind   Kind
		finite bool
		text   string
	}{
		{exact, Exact, true, "Exact(0)"},
		{upper, UpperBound, true, "UpperBound(8192)"},
		{Bound{}, Unknown, false, "Unknown"},
		{UnknownBound(), Unknown, false, "Unknown"},
		{UnboundedBound(), Unbounded, false, "Unbounded"},
	} {
		if test.bound.Kind() != test.kind || test.bound.String() != test.text {
			t.Fatalf("bound = %s (%s), want %s (%s)", test.bound, test.bound.Kind(), test.text, test.kind)
		}
		bytes, finite := test.bound.Bytes()
		if finite != test.finite || (bytes != nil) != test.finite {
			t.Fatalf("%s byte count = %v, %v", test.bound, bytes, finite)
		}
	}
	if UnknownBound() == UnboundedBound() || exact == UnknownBound() {
		t.Fatal("different proof qualities share a representation")
	}
	zeroUpper, err := NewUpper(big.NewInt(0))
	if err != nil || zeroUpper.Kind() != UpperBound || zeroUpper == exact {
		t.Fatal("zero upper bound lost its proof quality", zeroUpper, err)
	}
}

// TestStackBoundFiniteCounts validates finite bounds and checks arbitrary
// precision, defensive input/output ownership, value copies and stable identity.
// Rules: rules/analysis/stack_analysis.md — "Exact", "UpperBound", and "Unbounded";
// rules/compiler/compiler_analysis.md — immutable analysis results.
func TestStackBoundFiniteCounts(t *testing.T) {
	constructors := []struct {
		name string
		make func(*big.Int) (Bound, error)
	}{
		{"exact", NewExact},
		{"upper", NewUpper},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			for _, invalid := range []*big.Int{nil, big.NewInt(-1)} {
				bound, err := constructor.make(invalid)
				if err == nil || bound.Kind() != Unknown {
					t.Fatalf("invalid count produced a proof: %s, %v", bound, err)
				}
			}
			text := "340282366920938463463374607431768211456"
			input, ok := new(big.Int).SetString(text, 10)
			if !ok {
				t.Fatal("bad test count")
			}
			bound, err := constructor.make(input)
			if err != nil {
				t.Fatal(err)
			}
			copy := bound
			input.SetInt64(1)
			bytes, finite := bound.Bytes()
			if !finite || bytes.String() != text {
				t.Fatal("input alias or overflow", bound)
			}
			bytes.SetInt64(2)
			fresh, finite := copy.Bytes()
			if !finite || fresh.String() != text {
				t.Fatal("output alias", copy)
			}
			equivalent, err := constructor.make(fresh)
			if err != nil || equivalent != bound {
				t.Fatal("unstable immutable value identity")
			}
		})
	}
}
