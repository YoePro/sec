package sema

import "strings"

func parseDimension(unit string) Dimension {
	dimension := Dimension{Base: map[string]int{}}
	if unit == "" {
		return dimension
	}

	sign := 1
	for _, part := range strings.Split(unit, "/") {
		for _, factor := range strings.Split(part, "*") {
			factor = strings.TrimSpace(factor)
			if factor == "" {
				continue
			}

			dimension.Base[factor] += sign
			if dimension.Base[factor] == 0 {
				delete(dimension.Base, factor)
			}
		}

		sign = -1
	}

	return dimension
}

func (a *Analyzer) typeForDimension(kind TypeKind, dimension Dimension) Type {
	for _, typ := range a.types {
		if typ.Kind == kind && typ.Named && typ.Dimension.Equal(dimension) {
			return typ
		}
	}

	if dimension.IsZero() && kind == DecimalType {
		return a.types["decimal"]
	}

	return Type{Name: string(kind), Kind: kind, Dimension: dimension}
}
