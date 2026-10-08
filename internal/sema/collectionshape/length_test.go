package collectionshape

import (
	"math/big"
	"testing"
)

// TestBoundsPreserveExactSourceValues checks huge inherited bounds, operand
// immutability, empty intersections and the fixed-array extent contribution.
// Rules: rules/types/contracts.md — "Composition", "String and collection contracts".
func TestBoundsPreserveExactSourceValues(t *testing.T) {
	huge := new(big.Int).Lsh(big.NewInt(1), 200)
	original := new(big.Int).Set(huge)
	var bounds Bounds
	if !bounds.Add("minLen", huge) || !bounds.Add("exactLen", huge) || !bounds.Add("maxLen", huge) {
		t.Fatal("exact huge intersection rejected")
	}
	if huge.Cmp(original) != 0 {
		t.Fatal("source bound was mutated")
	}
	// Mutating a caller-owned operand cannot alter previously resolved facts.
	huge.SetInt64(0)
	if bounds.Add("maxLen", new(big.Int).Sub(original, big.NewInt(1))) {
		t.Fatal("empty inherited intersection accepted")
	}
	var empty Bounds
	if !empty.Add("exactLen", big.NewInt(0)) || empty.Add("notEmpty", nil) {
		t.Fatal("zero-length notEmpty intersection accepted")
	}
	var fixed Bounds
	if !fixed.Add("minLen", big.NewInt(5)) || fixed.Add("exactLen", big.NewInt(4)) {
		t.Fatal("fixed extent below minimum accepted")
	}
	if !Satisfies(original, "exactLen", original) || Satisfies(original, "maxLen", big.NewInt(0)) || Satisfies(big.NewInt(-1), "minLen", big.NewInt(0)) || Satisfies(big.NewInt(0), "notEmpty", nil) {
		t.Fatal("invalid exact length proof")
	}
}
