package sema

import (
	"math/big"
	"strconv"
	"testing"
)

// Open enums are bounded by their resolved bit width, never by named members
// or cached extrema that may describe only those members.
// Rules: rules/declarations/enums.md — §§2,10,12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func TestOpenEnumProofDerivesCompleteBitDomain(t *testing.T) {
	a := NewAnalyzer()
	for _, test := range []struct {
		width  int64
		target string
		exact  bool
	}{
		{1, "uint8", true}, {7, "int8", true}, {8, "int8", false},
		{8, "uint8", true}, {16, "uint8", false}, {16, "uint16", true},
		{64, "int64", false}, {64, "uint64", true},
		{256, "int256", false}, {256, "uint256", true},
	} {
		t.Run(strconv.FormatInt(test.width, 10)+"_to_"+test.target, func(t *testing.T) {
			source := Type{Kind: EnumType, Underlying: "bit[" + strconv.FormatInt(test.width, 10) + "]", BitWidth: test.width,
				EnumValues: []string{"Zero", "One"}, EnumConsts: builtinEnumConsts([]string{"Zero", "One"}),
				MinInteger: big.NewInt(0), MaxInteger: big.NewInt(1)}
			if test.width == 1 {
				source.Underlying = "bit"
			}
			for _, cachedBounds := range []bool{true, false} {
				if !cachedBounds {
					source.MinInteger, source.MaxInteger = nil, nil
				}
				proof := a.integerConversionProof(a.Types()[test.target], source, nil)
				if proof.Proven() != test.exact {
					t.Fatalf("cached=%v proof=%+v", cachedBounds, proof)
				}
				if test.exact {
					maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(test.width)), big.NewInt(1))
					if proof.Basis != IntegerConversionOpenEnumDomain || proof.SourceMin.Sign() != 0 || proof.SourceMax.Cmp(maximum) != 0 {
						t.Fatalf("named cases narrowed open domain: %+v", proof)
					}
				}
			}
		})
	}
}

// An unresolved/invalid open width must not be reclassified as a closed enum
// just because its declared cases happen to fit the target.
// Rules: rules/declarations/enums.md — §2 (1..256), §§10,12.1;
// rules/compiler/semantic_ir.md — §124 (fail closed).
func TestOpenEnumProofRejectsInvalidWidths(t *testing.T) {
	a := NewAnalyzer()
	for _, width := range []int64{-1, 0, 257} {
		source := Type{Kind: EnumType, Underlying: "bit[8]", BitWidth: width,
			EnumValues: []string{"Zero", "One"}, EnumConsts: builtinEnumConsts([]string{"Zero", "One"}),
			MinInteger: big.NewInt(0), MaxInteger: big.NewInt(1)}
		if proof := a.integerConversionProof(a.Types()["uint8"], source, nil); proof.Proven() {
			t.Fatalf("width %d manufactured proof: %+v", width, proof)
		}
	}
}
