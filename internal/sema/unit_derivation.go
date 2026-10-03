package sema

import (
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// UnitDerivationStep is one normalized factor of a unit expression with the
// facts of its declared unit.
type UnitDerivationStep struct {
	Unit      string
	Exponent  int
	Dimension string
	Scale     string
	Known     bool
}

// UnitDerivation explains how a quantity type's normalized dimension and
// exact scale follow from its unit factors, in source order.
type UnitDerivation struct {
	Steps     []UnitDerivationStep
	Dimension string
	Scale     string
}

// UnitDerivationOf exposes the dimension derivation of typ's unit expression
// from Sema's own unit definitions: every normalized factor with its declared
// dimension and scale, the resulting normalized dimension, and the exact
// combined scale when every factor's scale is known. Tooling presents it and
// never re-derives unit algebra.
//
// Rules:
//   - rules/types/units.md — "LSP requirements" (one shared semantic unit model)
//   - rules/tooling/lsp.md — "Unit actions" (Show dimension derivation)
func (a *Analyzer) UnitDerivationOf(typ Type) (UnitDerivation, bool) {
	unit := typ.UnitSemantics
	if unit.Identity == "" || len(unit.Factors) == 0 {
		return UnitDerivation{}, false
	}
	order := []string{}
	seen := map[string]bool{}
	for _, factor := range unit.SourceFactors {
		if exponent, ok := unit.Factors[factor]; ok && exponent != 0 && !seen[factor] {
			order = append(order, factor)
			seen[factor] = true
		}
	}
	remaining := []string{}
	for factor, exponent := range unit.Factors {
		if exponent != 0 && !seen[factor] {
			remaining = append(remaining, factor)
		}
	}
	sort.Strings(remaining)
	order = append(order, remaining...)

	derivation := UnitDerivation{Dimension: CanonicalDimensionDisplay(typ.Dimension)}
	combined := big.NewRat(1, 1)
	scaleKnown := true
	for _, name := range order {
		exponent := unit.Factors[name]
		step := UnitDerivationStep{Unit: name, Exponent: exponent}
		if definition, ok := a.units[name]; ok {
			step.Known = true
			step.Dimension = CanonicalDimensionDisplay(definition.Dimension)
			if definition.ScaleValue != nil {
				step.Scale = definition.ScaleValue.RatString()
				combined.Mul(combined, ratPower(definition.ScaleValue, exponent))
			} else {
				scaleKnown = false
			}
		} else {
			scaleKnown = false
		}
		derivation.Steps = append(derivation.Steps, step)
	}
	if scaleKnown {
		derivation.Scale = combined.RatString()
	}
	return derivation, true
}

// CanonicalDimensionDisplay renders a dimension in the canonical
// `[axis^exponent, ...]` form with axes in name order; an empty dimension is
// dimensionless.
//
// Rules:
//   - rules/types/units.md — "Formatter requirements" (canonical dimension syntax)
func CanonicalDimensionDisplay(dimension Dimension) string {
	axes := make([]string, 0, len(dimension.Base))
	for axis, exponent := range dimension.Base {
		if exponent != 0 {
			axes = append(axes, axis)
		}
	}
	if len(axes) == 0 {
		return "dimensionless"
	}
	sort.Strings(axes)
	parts := make([]string, len(axes))
	for index, axis := range axes {
		parts[index] = axis + "^" + strconv.Itoa(dimension.Base[axis])
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func ratPower(base *big.Rat, exponent int) *big.Rat {
	result := big.NewRat(1, 1)
	factor := new(big.Rat).Set(base)
	if exponent < 0 {
		if factor.Sign() == 0 {
			return result
		}
		factor.Inv(factor)
		exponent = -exponent
	}
	for ; exponent > 0; exponent-- {
		result.Mul(result, factor)
	}
	return result
}
