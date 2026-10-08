package stackbound

import (
	"math/big"
	"testing"
)

// TestStackRecursionDepthClassifications keeps the four proof qualities distinct
// and checks that neither uncertainty nor proven unboundedness carries a count.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification",
// "Unknown recursion depth", and "Proven unbounded recursion".
func TestStackRecursionDepthClassifications(t *testing.T) {
	exact, err := NewExactDepth(big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	upper, err := NewUpperDepth(big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		depth  Depth
		kind   DepthKind
		finite bool
		text   string
	}{
		{exact, DepthExact, true, "ExactDepth(0)"},
		{upper, DepthUpperBound, true, "UpperBoundDepth(0)"},
		{Depth{}, DepthUnknown, false, "UnknownDepth"},
		{UnknownDepth(), DepthUnknown, false, "UnknownDepth"},
		{UnboundedDepth(), DepthUnbounded, false, "UnboundedDepth"},
	} {
		if test.depth.Kind() != test.kind || test.depth.String() != test.text {
			t.Fatalf("got %s (%s), want %s (%s)", test.depth, test.depth.Kind(), test.text, test.kind)
		}
		count, finite := test.depth.Count()
		if finite != test.finite || (count != nil) != test.finite {
			t.Fatalf("%s count = %v, %v", test.depth, count, finite)
		}
	}
	if exact == upper || exact == UnknownDepth() || UnknownDepth() == UnboundedDepth() {
		t.Fatal("distinct depth proofs share identity")
	}
}

// TestStackRecursionDepthFiniteCounts rejects invalid proofs and verifies that
// finite depths exceeding uint64 remain immutable finite facts.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification";
// rules/compiler/compiler_analysis.md — immutable analysis results.
func TestStackRecursionDepthFiniteCounts(t *testing.T) {
	for _, constructor := range []struct {
		name string
		make func(*big.Int) (Depth, error)
	}{
		{"exact", NewExactDepth},
		{"upper", NewUpperDepth},
	} {
		t.Run(constructor.name, func(t *testing.T) {
			for _, invalid := range []*big.Int{nil, big.NewInt(-1)} {
				depth, err := constructor.make(invalid)
				if err == nil || depth != UnknownDepth() {
					t.Fatal("invalid count became a depth proof", depth, err)
				}
			}
			for _, input := range []*big.Int{big.NewInt(1), big.NewInt(42), new(big.Int).Lsh(big.NewInt(1), 128)} {
				want := input.String()
				depth, err := constructor.make(input)
				if err != nil {
					t.Fatal(err)
				}
				copy := depth
				input.SetInt64(0)
				count, finite := depth.Count()
				if !finite || count.String() != want {
					t.Fatal("input alias or count overflow", depth)
				}
				count.SetInt64(0)
				fresh, finite := copy.Count()
				if !finite || fresh.String() != want {
					t.Fatal("output alias", copy)
				}
				equivalent, err := constructor.make(fresh)
				if err != nil || equivalent != depth {
					t.Fatal("unstable depth identity", err)
				}
			}
		})
	}
}
