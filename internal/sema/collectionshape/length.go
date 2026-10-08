// Package collectionshape proves lengths of resolved collection values without
// depending on Analyzer state, source identities, or collection storage layout.
package collectionshape

import "math/big"

// Satisfies checks an exact semantic length without narrowing to host width.
// Rules: rules/types/contracts.md — "String and collection contracts".
func Satisfies(length *big.Int, name string, bound *big.Int) bool {
	if length == nil || length.Sign() < 0 {
		return false
	}
	if name == "notEmpty" {
		return length.Sign() > 0
	}
	if bound == nil || bound.Sign() < 0 {
		return false
	}
	switch name {
	case "minLen":
		return length.Cmp(bound) >= 0
	case "maxLen":
		return length.Cmp(bound) <= 0
	case "exactLen":
		return length.Cmp(bound) == 0
	default:
		return false
	}
}

// Bounds intersects inherited and local length requirements. The zero value
// denotes every nonnegative length; source operands are copied, never mutated.
// Rules: rules/types/contracts.md — "Composition", "String and collection contracts".
type Bounds struct {
	minimum big.Int
	maximum *big.Int
}

// Add intersects one canonical requirement and reports whether any length
// remains. A fixed-array extent contributes an exactLen requirement.
// Rules: rules/types/contracts.md — "Composition", "String and collection contracts".
func (b *Bounds) Add(name string, bound *big.Int) bool {
	if name == "notEmpty" {
		name, bound = "minLen", big.NewInt(1)
	}
	if bound == nil || bound.Sign() < 0 {
		return false
	}
	switch name {
	case "minLen", "exactLen":
		if bound.Cmp(&b.minimum) > 0 {
			b.minimum.Set(bound)
		}
	case "maxLen":
	default:
		return false
	}
	if name == "maxLen" || name == "exactLen" {
		if b.maximum == nil || bound.Cmp(b.maximum) < 0 {
			b.maximum = new(big.Int).Set(bound)
		}
	}
	return b.maximum == nil || b.minimum.Cmp(b.maximum) <= 0
}
