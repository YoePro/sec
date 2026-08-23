package sema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func dimensionFromBase(name string, exponent int) Dimension {
	if exponent == 0 {
		return Dimension{Base: map[string]int{}}
	}
	return Dimension{Base: map[string]int{name: exponent}}
}

// defaultUnitDimension implements the category defaults in rules/types/units.md.
// correction6.md distinguishes an unresolved physical dimension from known
// dimensionless and category-derived dimensions.
func defaultUnitDimension(name string, category UnitCategory) (Dimension, bool) {
	switch category {
	case RatioUnit:
		return Dimension{Base: map[string]int{}}, true
	case InformationUnit:
		return dimensionFromBase("information", 1), true
	case PhysicalUnit:
		return Dimension{Base: map[string]int{}}, false
	default:
		return dimensionFromBase(name, 1), true
	}
}

type unitExpressionParser struct {
	input []rune
	pos   int
	units map[string]UnitDefinition
}

func resolveUnitExpression(source string, units map[string]UnitDefinition) (Dimension, error) {
	parser := unitExpressionParser{input: []rune(source), units: units}
	dimension, err := parser.parseProduct()
	if err != nil {
		return Dimension{}, err
	}
	parser.skipSpace()
	if parser.pos != len(parser.input) {
		return Dimension{}, fmt.Errorf("unexpected %q", string(parser.input[parser.pos]))
	}
	return dimension, nil
}

func (p *unitExpressionParser) parseProduct() (Dimension, error) {
	left, err := p.parseFactor()
	if err != nil {
		return Dimension{}, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] != '*' && p.input[p.pos] != '/' {
			return left, nil
		}
		op := p.input[p.pos]
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return Dimension{}, err
		}
		if op == '/' {
			right = scaleDimension(right, -1)
		}
		left = addDimensions(left, right)
	}
}

func (p *unitExpressionParser) parseFactor() (Dimension, error) {
	dimension, err := p.parseAtom()
	if err != nil {
		return Dimension{}, err
	}
	p.skipSpace()
	if p.pos >= len(p.input) || p.input[p.pos] != '^' {
		return dimension, nil
	}
	p.pos++
	p.skipSpace()
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}
	for p.pos < len(p.input) && unicode.IsDigit(p.input[p.pos]) {
		p.pos++
	}
	if start == p.pos || p.pos == start+1 && (p.input[start] == '+' || p.input[start] == '-') {
		return Dimension{}, fmt.Errorf("unit exponent must be a signed integer")
	}
	exponent, err := strconv.Atoi(string(p.input[start:p.pos]))
	if err != nil || exponent == 0 {
		return Dimension{}, fmt.Errorf("unit exponent must be a non-zero signed integer")
	}
	return scaleDimension(dimension, exponent), nil
}

func (p *unitExpressionParser) parseAtom() (Dimension, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return Dimension{}, fmt.Errorf("expected unit factor")
	}
	if p.input[p.pos] == '(' {
		p.pos++
		dimension, err := p.parseProduct()
		if err != nil {
			return Dimension{}, err
		}
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return Dimension{}, fmt.Errorf("expected ')' in unit expression")
		}
		p.pos++
		return dimension, nil
	}
	if p.input[p.pos] == '1' {
		p.pos++
		return Dimension{Base: map[string]int{}}, nil
	}
	name, ok := p.parseQualifiedIdentifier()
	if !ok {
		return Dimension{}, fmt.Errorf("expected a unit name, 1, or parenthesized unit expression")
	}
	unit, exists := p.units[name]
	if !exists {
		return Dimension{}, fmt.Errorf("unknown unit %s", name)
	}
	return unit.Dimension, nil
}

func (p *unitExpressionParser) parseQualifiedIdentifier() (string, bool) {
	start := p.pos
	for {
		if p.pos >= len(p.input) || !isUnitIdentifierStart(p.input[p.pos]) {
			return "", false
		}
		p.pos++
		for p.pos < len(p.input) && isUnitIdentifierContinue(p.input[p.pos]) {
			p.pos++
		}
		if p.pos >= len(p.input) || p.input[p.pos] != '.' {
			break
		}
		p.pos++
	}
	return string(p.input[start:p.pos]), true
}

func (p *unitExpressionParser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func isUnitIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isUnitIdentifierContinue(r rune) bool {
	return isUnitIdentifierStart(r) || unicode.IsDigit(r)
}

func addDimensions(left, right Dimension) Dimension {
	result := Dimension{Base: map[string]int{}}
	for name, exponent := range left.Base {
		result.Base[name] = exponent
	}
	for name, exponent := range right.Base {
		result.Base[name] += exponent
		if result.Base[name] == 0 {
			delete(result.Base, name)
		}
	}
	return result
}

func scaleDimension(dimension Dimension, scale int) Dimension {
	result := Dimension{Base: map[string]int{}}
	for name, exponent := range dimension.Base {
		if scaled := exponent * scale; scaled != 0 {
			result.Base[name] = scaled
		}
	}
	return result
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
