package sema

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// resolveUnitSemantics consumes the first-class unit AST required by
// rules/types/units.md. The legacy source string is retained only as exact
// spelling/provenance and is never the semantic identity discriminator.
func resolveUnitSemantics(source string, expression *ast.UnitExpression, units map[string]UnitDefinition) (UnitSemantics, Dimension, error) {
	if expression == nil {
		dimension, err := resolveUnitExpression(source, units)
		if err != nil {
			return UnitSemantics{}, Dimension{}, err
		}
		return UnitSemantics{
			Identity: StructuralUnitIdentity, Source: source,
			Factors: map[string]int{}, Categories: map[UnitCategory]bool{},
			Transform: LinearUnitTransform, Scale: big.NewRat(1, 1), Role: UnitVectorRole,
		}, dimension, nil
	}
	semantics, dimension, err := resolveUnitSemanticsNode(expression, units)
	if err != nil {
		return UnitSemantics{}, Dimension{}, err
	}
	semantics.Source = source
	if semantics.Scale == nil {
		semantics.Scale = big.NewRat(1, 1)
	}
	return semantics, dimension, nil
}

func resolveNamedUnitSemantics(name string, units map[string]UnitDefinition) (UnitSemantics, Dimension, error) {
	return resolveUnitSemantics(name, &ast.UnitExpression{Kind: ast.UnitExpressionName, Name: name}, units)
}

func resolveUnitSemanticsNode(expression *ast.UnitExpression, units map[string]UnitDefinition) (UnitSemantics, Dimension, error) {
	if expression == nil {
		return UnitSemantics{}, Dimension{}, fmt.Errorf("expected unit expression")
	}
	switch expression.Kind {
	case ast.UnitExpressionName:
		unit, ok := units[expression.Name]
		if !ok {
			return UnitSemantics{}, Dimension{}, fmt.Errorf("unknown unit %s", expression.Name)
		}
		scale := cloneRat(unit.ScaleValue)
		if scale == nil {
			scale = big.NewRat(1, 1)
		}
		role := UnitVectorRole
		if unit.Origin != "" {
			role = UnitPointRolePoint
		}
		return UnitSemantics{
			Identity: NamedUnitIdentity, Named: expression.Name,
			SourceFactors: []string{expression.Name}, Factors: map[string]int{expression.Name: 1},
			Categories: map[UnitCategory]bool{unit.Category: true}, Kind: unit.Kind,
			Transform: unit.Transform, Scale: scale, Offset: cloneRat(unit.OffsetValue), Origin: unit.Origin,
			LogBase: cloneRat(unit.LogBaseValue), LogFactor: cloneRat(unit.LogFactorValue), Reference: unit.Reference,
			Role: role, ConversionExact: true,
		}, unit.Dimension, nil
	case ast.UnitExpressionIdentity:
		return UnitSemantics{
			Identity: StructuralUnitIdentity, SourceFactors: []string{"1"}, Factors: map[string]int{},
			Categories: map[UnitCategory]bool{}, Transform: LinearUnitTransform,
			Scale: big.NewRat(1, 1), Role: UnitVectorRole, ConversionExact: true,
		}, Dimension{Base: map[string]int{}}, nil
	case ast.UnitExpressionGroup:
		semantics, dimension, err := resolveUnitSemanticsNode(expression.Left, units)
		semantics.Identity = StructuralUnitIdentity
		semantics.Named = ""
		return semantics, dimension, err
	case ast.UnitExpressionPower:
		semantics, dimension, err := resolveUnitSemanticsNode(expression.Left, units)
		if err != nil {
			return UnitSemantics{}, Dimension{}, err
		}
		semantics.Identity = StructuralUnitIdentity
		semantics.Named = ""
		semantics.Factors = scaleUnitFactors(semantics.Factors, expression.Exponent)
		semantics.SourceFactors = append([]string(nil), semantics.SourceFactors...)
		semantics.Kind = ""
		semantics.Transform = LinearUnitTransform
		semantics.Offset = nil
		semantics.Origin = ""
		semantics.Role = UnitVectorRole
		semantics.Scale = powRat(semantics.Scale, expression.Exponent)
		return semantics, scaleDimension(dimension, expression.Exponent), nil
	case ast.UnitExpressionMultiply, ast.UnitExpressionDivide:
		left, leftDimension, err := resolveUnitSemanticsNode(expression.Left, units)
		if err != nil {
			return UnitSemantics{}, Dimension{}, err
		}
		right, rightDimension, err := resolveUnitSemanticsNode(expression.Right, units)
		if err != nil {
			return UnitSemantics{}, Dimension{}, err
		}
		sign := 1
		if expression.Kind == ast.UnitExpressionDivide {
			sign = -1
		}
		result := UnitSemantics{
			Identity:      StructuralUnitIdentity,
			SourceFactors: append(append([]string(nil), left.SourceFactors...), right.SourceFactors...),
			Factors:       combineUnitFactors(left.Factors, right.Factors, sign),
			Categories:    combineUnitCategories(left.Categories, right.Categories),
			Transform:     LinearUnitTransform, Scale: combineRat(left.Scale, right.Scale, sign),
			Role: UnitVectorRole, ConversionExact: true,
		}
		// A linear dimensionless ratio scales another quantity without erasing
		// that quantity's known Kind. Other products are semantically ambiguous.
		if leftDimension.IsZero() && isLinearRatioSemantics(left) {
			result.Kind = right.Kind
		} else if rightDimension.IsZero() && isLinearRatioSemantics(right) {
			result.Kind = left.Kind
		}
		if sign < 0 {
			rightDimension = scaleDimension(rightDimension, -1)
		}
		return result, addDimensions(leftDimension, rightDimension), nil
	default:
		return UnitSemantics{}, Dimension{}, fmt.Errorf("unsupported unit expression kind %s", expression.Kind)
	}
}

func cloneRat(value *big.Rat) *big.Rat {
	if value == nil {
		return nil
	}
	return new(big.Rat).Set(value)
}

func powRat(value *big.Rat, exponent int) *big.Rat {
	if value == nil {
		value = big.NewRat(1, 1)
	}
	result := big.NewRat(1, 1)
	base := cloneRat(value)
	power := exponent
	if power < 0 {
		base.Inv(base)
		power = -power
	}
	for power > 0 {
		if power&1 == 1 {
			result.Mul(result, base)
		}
		base.Mul(base, base)
		power >>= 1
	}
	return result
}

func combineRat(left, right *big.Rat, sign int) *big.Rat {
	if left == nil {
		left = big.NewRat(1, 1)
	}
	if right == nil {
		right = big.NewRat(1, 1)
	}
	result := cloneRat(left)
	if sign < 0 {
		return result.Quo(result, right)
	}
	return result.Mul(result, right)
}

func scaleUnitFactors(factors map[string]int, exponent int) map[string]int {
	result := map[string]int{}
	for name, value := range factors {
		if scaled := value * exponent; scaled != 0 {
			result[name] = scaled
		}
	}
	return result
}

func combineUnitFactors(left, right map[string]int, sign int) map[string]int {
	result := scaleUnitFactors(left, 1)
	for name, exponent := range right {
		result[name] += sign * exponent
		if result[name] == 0 {
			delete(result, name)
		}
	}
	return result
}

func combineUnitCategories(left, right map[UnitCategory]bool) map[UnitCategory]bool {
	result := map[UnitCategory]bool{}
	for category := range left {
		result[category] = true
	}
	for category := range right {
		result[category] = true
	}
	return result
}

func isLinearRatioSemantics(semantics UnitSemantics) bool {
	return semantics.Transform == "" || semantics.Transform == LinearUnitTransform
}

func hasUnitSemantics(typ Type) bool {
	return typ.Unit != "" || typ.UnitSemantics.Identity != ""
}

// effectiveUnitSemantics bridges legacy synthesized quantity types into the
// explicit unit model required by rules/types/units.md, "Semantic IR
// requirements". New source types arrive with a complete descriptor.
func effectiveUnitSemantics(typ Type) UnitSemantics {
	if typ.UnitSemantics.Identity != "" {
		return typ.UnitSemantics
	}
	if typ.Unit == "" {
		return scalarUnitSemantics()
	}
	identity := StructuralUnitIdentity
	named := ""
	if typ.Named && typ.Unit == typ.Name {
		identity = NamedUnitIdentity
		named = typ.Unit
	}
	return UnitSemantics{
		Identity: identity, Named: named, Source: typ.Unit,
		SourceFactors: []string{typ.Unit}, Factors: map[string]int{typ.Unit: 1},
		Categories: map[UnitCategory]bool{}, Transform: LinearUnitTransform,
		Scale: big.NewRat(1, 1), Role: UnitVectorRole, ConversionExact: true,
	}
}

func unitKindCompatible(left, right UnitSemantics) bool {
	if left.Kind != "" && right.Kind != "" {
		return left.Kind == right.Kind
	}
	// Unknown Kind is not evidence that different named units are compatible.
	if left.Identity == NamedUnitIdentity && right.Identity == NamedUnitIdentity && left.Named != right.Named {
		return false
	}
	return true
}

func unitOriginCompatible(left, right UnitSemantics) bool {
	if left.Role != UnitPointRolePoint && right.Role != UnitPointRolePoint {
		return true
	}
	return left.Role == UnitPointRolePoint && right.Role == UnitPointRolePoint && left.Origin != "" && left.Origin == right.Origin
}

func unitScaleRatio(source, target UnitSemantics) *big.Rat {
	sourceScale := source.Scale
	if sourceScale == nil {
		sourceScale = big.NewRat(1, 1)
	}
	targetScale := target.Scale
	if targetScale == nil {
		targetScale = big.NewRat(1, 1)
	}
	return new(big.Rat).Quo(sourceScale, targetScale)
}

func exactImplicitUnitConversion(source, target UnitSemantics, carrier TypeKind) bool {
	if source.Transform == LogarithmicUnitTransform || target.Transform == LogarithmicUnitTransform {
		return source.Identity == target.Identity && source.Named == target.Named
	}
	ratio := unitScaleRatio(source, target)
	if source.Role == UnitPointRolePoint || target.Role == UnitPointRolePoint {
		if !unitOriginCompatible(source, target) {
			return false
		}
		sourceOffset := source.Offset
		if sourceOffset == nil {
			sourceOffset = big.NewRat(0, 1)
		}
		targetOffset := target.Offset
		if targetOffset == nil {
			targetOffset = big.NewRat(0, 1)
		}
		// target coordinate = (source*sourceScale + sourceOffset-targetOffset)/targetScale
		offset := new(big.Rat).Quo(new(big.Rat).Sub(sourceOffset, targetOffset), target.Scale)
		if carrier != DecimalType && offset.Sign() != 0 {
			return false
		}
	}
	switch carrier {
	case DecimalType:
		return finiteDecimalRat(ratio)
	case FloatType, IntType, UintType:
		// Without a value range/proof, only an identity coordinate conversion
		// can promise no rounding, truncation, precision loss, or overflow.
		return ratio.Cmp(big.NewRat(1, 1)) == 0
	default:
		return false
	}
}

func finiteDecimalRat(value *big.Rat) bool {
	if value == nil {
		return true
	}
	denominator := new(big.Int).Set(value.Denom())
	for _, prime := range []int64{2, 5} {
		p := big.NewInt(prime)
		for new(big.Int).Mod(denominator, p).Sign() == 0 {
			denominator.Quo(denominator, p)
		}
	}
	return denominator.Cmp(big.NewInt(1)) == 0
}

func combineQuantitySemantics(left, right UnitSemantics, operator string) UnitSemantics {
	sign := 1
	if operator == "/" {
		sign = -1
	}
	result := UnitSemantics{
		Identity:        StructuralUnitIdentity,
		Source:          left.Source + operator + right.Source,
		SourceFactors:   append(append([]string(nil), left.SourceFactors...), right.SourceFactors...),
		Factors:         combineUnitFactors(left.Factors, right.Factors, sign),
		Categories:      combineUnitCategories(left.Categories, right.Categories),
		Transform:       LinearUnitTransform,
		Scale:           combineRat(left.Scale, right.Scale, sign),
		Role:            UnitVectorRole,
		ConversionExact: true,
	}
	leftDimensionless := len(left.Factors) == 0
	rightDimensionless := len(right.Factors) == 0
	if leftDimensionless && isLinearRatioSemantics(left) {
		result.Kind = right.Kind
	} else if rightDimensionless && isLinearRatioSemantics(right) {
		result.Kind = left.Kind
	}
	return result
}

func scalarUnitSemantics() UnitSemantics {
	return UnitSemantics{
		Identity: StructuralUnitIdentity, Source: "1", SourceFactors: []string{"1"},
		Factors: map[string]int{}, Categories: map[UnitCategory]bool{},
		Transform: LinearUnitTransform, Scale: big.NewRat(1, 1), Role: UnitVectorRole,
		ConversionExact: true,
	}
}

func unitResultType(carrier Type, semantics UnitSemantics, dimension Dimension) Type {
	result := carrier
	result.Named = false
	result.Declared = false
	result.Underlying = ""
	result.Dimension = dimension
	result.UnitSemantics = semantics
	if dimension.IsZero() && len(semantics.Factors) == 0 {
		result.Unit = ""
		result.UnitSemantics.Source = "1"
		return result
	}
	result.Unit = semantics.Source
	return result
}

// exactUnitMetadataValue evaluates the closed exact numeric subset used by
// rules/types/units.md for Scale, Offset, LogBase, and LogFactor. Keeping this
// as big.Rat prevents frontend conversion planning from inheriting host float
// rounding.
func exactUnitMetadataValue(tokens []lexer.Token) (*big.Rat, bool) {
	p := exactUnitValueParser{tokens: tokens}
	value, ok := p.parseSum()
	return value, ok && p.pos == len(tokens)
}

type exactUnitValueParser struct {
	tokens []lexer.Token
	pos    int
}

func (p *exactUnitValueParser) parseSum() (*big.Rat, bool) {
	left, ok := p.parseProduct()
	if !ok {
		return nil, false
	}
	for p.pos < len(p.tokens) && (p.tokens[p.pos].Type == lexer.PLUS || p.tokens[p.pos].Type == lexer.MINUS) {
		op := p.tokens[p.pos].Type
		p.pos++
		right, ok := p.parseProduct()
		if !ok {
			return nil, false
		}
		if op == lexer.PLUS {
			left.Add(left, right)
		} else {
			left.Sub(left, right)
		}
	}
	return left, true
}

func (p *exactUnitValueParser) parseProduct() (*big.Rat, bool) {
	left, ok := p.parsePower()
	if !ok {
		return nil, false
	}
	for p.pos < len(p.tokens) && (p.tokens[p.pos].Type == lexer.ASTERISK || p.tokens[p.pos].Type == lexer.SLASH) {
		op := p.tokens[p.pos].Type
		p.pos++
		right, ok := p.parsePower()
		if !ok || op == lexer.SLASH && right.Sign() == 0 {
			return nil, false
		}
		if op == lexer.ASTERISK {
			left.Mul(left, right)
		} else {
			left.Quo(left, right)
		}
	}
	return left, true
}

func (p *exactUnitValueParser) parsePower() (*big.Rat, bool) {
	value, ok := p.parseAtom()
	if !ok || p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.BIT_XOR {
		return value, ok
	}
	p.pos++
	sign := 1
	if p.pos < len(p.tokens) && (p.tokens[p.pos].Type == lexer.PLUS || p.tokens[p.pos].Type == lexer.MINUS) {
		if p.tokens[p.pos].Type == lexer.MINUS {
			sign = -1
		}
		p.pos++
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.INT {
		return nil, false
	}
	exponent, err := strconv.Atoi(p.tokens[p.pos].Lexeme)
	if err != nil {
		return nil, false
	}
	p.pos++
	return powRat(value, sign*exponent), true
}

func (p *exactUnitValueParser) parseAtom() (*big.Rat, bool) {
	if p.pos >= len(p.tokens) {
		return nil, false
	}
	token := p.tokens[p.pos]
	if token.Type == lexer.PLUS || token.Type == lexer.MINUS {
		p.pos++
		value, ok := p.parseAtom()
		if ok && token.Type == lexer.MINUS {
			value.Neg(value)
		}
		return value, ok
	}
	if token.Type == lexer.LPAREN {
		p.pos++
		value, ok := p.parseSum()
		if !ok || p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.RPAREN {
			return nil, false
		}
		p.pos++
		return value, true
	}
	if token.Type != lexer.INT && token.Type != lexer.FLOAT {
		return nil, false
	}
	p.pos++
	lexeme := strings.TrimRightFunc(token.Lexeme, func(r rune) bool { return unicode.IsLetter(r) })
	value, ok := new(big.Rat).SetString(lexeme)
	return value, ok
}

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
