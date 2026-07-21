package sema

import (
	"sort"
	"strings"
)

func builtinUnits() map[string]UnitDefinition {
	units := map[string]UnitDefinition{}

	addPhysical := func(names []string, dimension Dimension) {
		for _, name := range names {
			units[name] = UnitDefinition{Name: name, Category: PhysicalUnit, Dimension: dimension}
		}
	}

	addPhysical([]string{"m", "metre", "meter"}, dimensionFromBase("length", 1))
	addPhysical([]string{"mm", "millimetre", "millimeter"}, dimensionFromBase("length", 1))
	addPhysical([]string{"inch"}, dimensionFromBase("length", 1))
	addPhysical([]string{"s", "second"}, dimensionFromBase("time", 1))
	addPhysical([]string{"kg", "kilogram"}, dimensionFromBase("mass", 1))
	addPhysical([]string{"Hz", "Hertz", "hertz"}, dimensionFromBase("time", -1))
	addPhysical([]string{"rpm"}, Dimension{Base: map[string]int{"revolution": 1, "time": -1}})

	return units
}

func dimensionFromBase(name string, exponent int) Dimension {
	if exponent == 0 {
		return Dimension{Base: map[string]int{}}
	}
	return Dimension{Base: map[string]int{name: exponent}}
}

func parseDimension(unit string) Dimension {
	return parseDimensionWithUnits(unit, nil)
}

func (a *Analyzer) parseDimension(unit string) Dimension {
	return parseDimensionWithUnits(unit, a.units)
}

func parseDimensionWithUnits(unit string, units map[string]UnitDefinition) Dimension {
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

			factorDimension := dimensionForFactor(factor, units)
			for base, exponent := range factorDimension.Base {
				dimension.Base[base] += sign * exponent
				if dimension.Base[base] == 0 {
					delete(dimension.Base, base)
				}
			}
		}

		sign = -1
	}

	return dimension
}

func dimensionForFactor(factor string, units map[string]UnitDefinition) Dimension {
	if units != nil {
		if unit, ok := units[factor]; ok {
			return unit.Dimension
		}
	}
	return Dimension{Base: map[string]int{factor: 1}}
}

func (a *Analyzer) typeForDimension(kind TypeKind, dimension Dimension) Type {
	if dimension.IsZero() && kind == DecimalType {
		return a.types["decimal"]
	}

	names := make([]string, 0, len(a.types))
	for name := range a.types {
		names = append(names, name)
	}
	sort.Strings(names)

	var unitMatch *Type
	for _, name := range names {
		typ := a.types[name]
		if typ.Kind != kind || !typ.Named || !typ.Dimension.Equal(dimension) {
			continue
		}
		if typ.Unit == typ.Name {
			copy := typ
			unitMatch = &copy
			continue
		}
		return typ
	}
	if unitMatch != nil {
		return *unitMatch
	}

	return Type{Name: string(kind), Kind: kind, Dimension: dimension}
}
