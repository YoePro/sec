// Package stringcontract owns immutable string-length requirements independently
// of Sema bindings, backend representation and allocation policy.
package stringcontract

import (
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"sec/internal/sema/collectionshape"
)

// Requirement retains the source-ordered contract identity and exact bound.
// Rules: rules/types/contracts.md — Composition, Public conversion and contract errors.
type Requirement struct {
	Kind             string
	Bound            string
	DeclarationIndex uint64
}

type rule struct {
	Requirement
	bound *big.Int
}
type Plan struct{ rules []rule }
type Violation struct {
	Kind             string
	DeclarationIndex uint64
}

// New snapshots exact nonnegative bounds; unsupported families are never erased.
// Rules: rules/types/contracts.md — String and collection contracts (revision 2.1).
func New(requirements []Requirement) (Plan, error) {
	p := Plan{}
	for _, requirement := range requirements {
		name := strings.Replace(requirement.Kind, "ByteLen", "Len", 1)
		switch name {
		case "minLen", "maxLen", "exactLen", "notEmpty":
		default:
			return Plan{}, fmt.Errorf("unsupported string contract %q", requirement.Kind)
		}
		var bound *big.Int
		if name != "notEmpty" {
			var ok bool
			bound, ok = new(big.Int).SetString(requirement.Bound, 10)
			if !ok || bound.Sign() < 0 {
				return Plan{}, fmt.Errorf("invalid string length bound %q", requirement.Bound)
			}
			requirement.Bound = bound.String()
		}
		p.rules = append(p.rules, rule{Requirement: requirement, bound: bound})
	}
	return p, nil
}

// Requirements returns independent immutable transport facts in source order.
// Rules: rules/types/contracts.md — Composition, Conversion failure layers.
func (p Plan) Requirements() []Requirement {
	result := make([]Requirement, len(p.rules))
	for i, r := range p.rules {
		result[i] = r.Requirement
	}
	return result
}

// Validate checks a runtime string once per length unit and reports only the
// first failed contract, retaining its original declaration index.
// Rules: rules/types/contracts.md — String and collection contracts, Public conversion and contract errors.
func (p Plan) Validate(text string) (Violation, bool) {
	runes, bytes := -1, len(text)
	for _, r := range p.rules {
		count := bytes
		name := strings.Replace(r.Kind, "ByteLen", "Len", 1)
		if !strings.Contains(r.Kind, "ByteLen") {
			if runes < 0 {
				runes = utf8.RuneCountInString(text)
			}
			count = runes
		}
		if !collectionshape.Satisfies(new(big.Int).SetUint64(uint64(count)), name, r.bound) {
			return Violation{Kind: r.Kind, DeclarationIndex: r.DeclarationIndex}, false
		}
	}
	return Violation{}, true
}

// Satisfiable proves rune/byte conjunctions using R <= B <= 4R for UTF-8;
// empty strings require R == B == 0. No host-width bound narrowing occurs.
// Rules: rules/types/contracts.md — Composition, String and collection contracts.
func (p Plan) Satisfiable() bool {
	runeMin, byteMin := new(big.Int), new(big.Int)
	var runeMax, byteMax *big.Int
	intersect := func(minimum *big.Int, maximum **big.Int, name string, bound *big.Int) {
		if name == "notEmpty" {
			name = "minLen"
			bound = big.NewInt(1)
		}
		if name == "minLen" || name == "exactLen" {
			if bound.Cmp(minimum) > 0 {
				minimum.Set(bound)
			}
		}
		if name == "maxLen" || name == "exactLen" {
			if *maximum == nil || bound.Cmp(*maximum) < 0 {
				*maximum = new(big.Int).Set(bound)
			}
		}
	}
	for _, r := range p.rules {
		if strings.Contains(r.Kind, "ByteLen") {
			intersect(byteMin, &byteMax, strings.Replace(r.Kind, "ByteLen", "Len", 1), r.bound)
		} else {
			intersect(runeMin, &runeMax, r.Kind, r.bound)
		}
	}
	// ceil(byteMin / 4) is the least possible rune count.
	fromBytes := new(big.Int).Quo(new(big.Int).Add(byteMin, big.NewInt(3)), big.NewInt(4))
	if fromBytes.Cmp(runeMin) > 0 {
		runeMin.Set(fromBytes)
	}
	if byteMax != nil && (runeMax == nil || byteMax.Cmp(runeMax) < 0) {
		runeMax = new(big.Int).Set(byteMax)
	}
	if runeMax != nil && runeMin.Cmp(runeMax) > 0 {
		return false
	}
	if byteMax != nil && byteMin.Cmp(byteMax) > 0 {
		return false
	}
	return true
}
