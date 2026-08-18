package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// DefaultValueOf resolves a compile-time, allocation-free semantic default.
func DefaultValueOf(typ Type) DefaultResolution {
	return defaultValueOf(typ, map[string]bool{})
}

func IsDefaultable(typ Type) bool { return DefaultValueOf(typ).Kind != NoDefault }

func DefaultValueDisplay(typ Type) (string, DefaultKind, bool) {
	resolution := DefaultValueOf(typ)
	if resolution.Kind == NoDefault {
		return "", NoDefault, false
	}
	return defaultResolutionDisplay(typ, resolution), resolution.Kind, true
}

// DefaultValuePreview returns a bounded presentation of a semantic default.
// It avoids materializing every element solely to render large fixed arrays in
// interactive tooling. DefaultValueOf remains the exact construction model.
func DefaultValuePreview(typ Type, maxArrayElements int) (string, DefaultKind, bool) {
	if typ.Kind != ArrayType {
		return DefaultValueDisplay(typ)
	}
	if typ.Element == nil {
		return "", NoDefault, false
	}
	if typ.ArrayLength == dynamicArrayLength {
		return "[]", ArrayDefault, true
	}
	if typ.ArrayLength < 0 {
		return "", NoDefault, false
	}
	if typ.ArrayLength == 0 {
		return "[]", ArrayDefault, true
	}

	element, _, ok := DefaultValuePreview(*typ.Element, maxArrayElements)
	if !ok {
		return "", NoDefault, false
	}
	if maxArrayElements < 1 {
		maxArrayElements = 1
	}
	if typ.ArrayLength > int64(maxArrayElements) {
		return "[" + element + ", ...]", ArrayDefault, true
	}

	elements := make([]string, int(typ.ArrayLength))
	for index := range elements {
		elements[index] = element
	}
	return "[" + strings.Join(elements, ", ") + "]", ArrayDefault, true
}

func defaultResolutionDisplay(typ Type, resolution DefaultResolution) string {
	switch resolution.Kind {
	case PrimitiveDefault, NamedDefault, RangeDefault, MembershipDefault, ExplicitTypeDefault:
		return resolution.Value.Lexeme
	case EnumDefault:
		return typ.Name + "." + resolution.Value.Lexeme
	case UnionDefault:
		if resolution.Payload != nil {
			return typ.Name + "." + resolution.Variant + "(" + defaultResolutionDisplay(unionDefaultPayloadType(typ, resolution.Variant), *resolution.Payload) + ")"
		}
		if len(resolution.Fields) > 0 {
			variant, _ := unionVariantByName(typ, resolution.Variant)
			parts := make([]string, 0, len(resolution.Fields))
			for _, field := range resolution.Fields {
				fieldType, ok := unionPayloadFieldType(variant, field.Name)
				if ok {
					parts = append(parts, field.Name+": "+defaultResolutionDisplay(fieldType, field.Value))
				}
			}
			return typ.Name + "." + resolution.Variant + " { " + strings.Join(parts, ", ") + " }"
		}
		return typ.Name + "." + resolution.Variant
	case StructDefault:
		parts := make([]string, 0, len(resolution.Fields))
		for _, field := range resolution.Fields {
			fieldType, ok := lookupStructField(typ, field.Name)
			if !ok {
				continue
			}
			parts = append(parts, field.Name+": "+defaultResolutionDisplay(fieldType, field.Value))
		}
		return typ.Name + " { " + strings.Join(parts, ", ") + " }"
	case ArrayDefault:
		parts := make([]string, 0, len(resolution.Elements))
		if typ.Element != nil {
			for _, element := range resolution.Elements {
				parts = append(parts, defaultResolutionDisplay(*typ.Element, element))
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return ""
	}
}

func unionVariantByName(typ Type, name string) (UnionVariant, bool) {
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return variant, true
		}
	}
	return UnionVariant{}, false
}

func unionDefaultPayloadType(typ Type, variantName string) Type {
	variant, ok := unionVariantByName(typ, variantName)
	if !ok || variant.Payload == nil {
		return Type{Kind: InvalidType}
	}
	return *variant.Payload
}

func unionPayloadFieldType(variant UnionVariant, name string) (Type, bool) {
	for _, field := range variant.PayloadFields {
		if field.Name == name {
			return field.Type, true
		}
	}
	return Type{}, false
}

func defaultValueOf(typ Type, visiting map[string]bool) DefaultResolution {
	if typ.InvalidExplicitDefault {
		return DefaultResolution{Kind: NoDefault}
	}
	if typ.ExplicitDefault != nil {
		return DefaultResolution{Kind: ExplicitTypeDefault, Value: *typ.ExplicitDefault}
	}
	for _, contract := range typ.Contracts {
		if membership, ok := contract.(MembershipContract); ok {
			for _, value := range membership.Values {
				if defaultConstantCompatible(typ, value) && defaultConstantSatisfies(typ, value) {
					return DefaultResolution{Kind: MembershipDefault, Value: value}
				}
			}
			return DefaultResolution{Kind: NoDefault}
		}
	}

	switch typ.Kind {
	case BoolType:
		return scalarDefault(typ, DefaultConstant{Kind: BoolType, Lexeme: "false"})
	case StringType:
		return scalarDefault(typ, DefaultConstant{Kind: StringType, Lexeme: `""`, String: ""})
	case CharType:
		return scalarDefault(typ, DefaultConstant{Kind: CharType, Lexeme: "0t", Integer: big.NewInt(0)})
	case RuneType:
		return scalarDefault(typ, DefaultConstant{Kind: RuneType, Lexeme: "0r", Integer: big.NewInt(0)})
	case IntType, UintType:
		value, kind, ok := integerDefault(typ)
		if !ok {
			return DefaultResolution{Kind: NoDefault}
		}
		return DefaultResolution{Kind: kindForNamed(typ, kind), Value: DefaultConstant{Kind: typ.Kind, Lexeme: value.String(), Integer: value}}
	case FloatType:
		zero := DefaultConstant{Kind: typ.Kind, Lexeme: "0.0", Exact: new(big.Rat)}
		if !defaultConstantSatisfies(typ, zero) {
			return DefaultResolution{Kind: NoDefault}
		}
		return scalarDefault(typ, zero)
	case DecimalType:
		return decimalDefault(typ)
	case EnumType:
		member := typ.EnumDefault
		if member == "" && len(typ.EnumValues) > 0 {
			member = typ.EnumValues[0]
		}
		if member == "" {
			return DefaultResolution{Kind: NoDefault}
		}
		return DefaultResolution{Kind: EnumDefault, Value: DefaultConstant{Kind: EnumType, Lexeme: member}}
	case UnionType:
		if typ.UnionDefault == "" {
			return DefaultResolution{Kind: NoDefault}
		}
		key := typ.Module + "." + typ.Name
		if visiting[key] {
			return DefaultResolution{Kind: NoDefault}
		}
		visiting[key] = true
		defer delete(visiting, key)
		variant, ok := unionVariantByName(typ, typ.UnionDefault)
		if !ok {
			return DefaultResolution{Kind: NoDefault}
		}
		result := DefaultResolution{Kind: UnionDefault, Variant: variant.Name}
		if variant.Payload != nil {
			payload := defaultValueOf(*variant.Payload, visiting)
			if payload.Kind == NoDefault {
				return DefaultResolution{Kind: NoDefault}
			}
			result.Payload = &payload
		}
		for _, field := range variant.PayloadFields {
			value := defaultValueOf(field.Type, visiting)
			if value.Kind == NoDefault {
				return DefaultResolution{Kind: NoDefault}
			}
			result.Fields = append(result.Fields, DefaultField{Name: field.Name, Value: value})
		}
		return result
	case ArrayType:
		if typ.Element == nil {
			return DefaultResolution{Kind: NoDefault}
		}
		if typ.ArrayLength == dynamicArrayLength {
			return DefaultResolution{Kind: ArrayDefault}
		}
		if typ.ArrayLength < 0 {
			return DefaultResolution{Kind: NoDefault}
		}
		if typ.ArrayLength == 0 {
			return DefaultResolution{Kind: ArrayDefault}
		}
		element := defaultValueOf(*typ.Element, visiting)
		if element.Kind == NoDefault {
			return DefaultResolution{Kind: NoDefault}
		}
		result := DefaultResolution{Kind: ArrayDefault}
		for i := int64(0); i < typ.ArrayLength; i++ {
			result.Elements = append(result.Elements, element)
		}
		return result
	case StructType:
		if typ.Intrinsic {
			return DefaultResolution{Kind: NoDefault}
		}
		key := typ.Module + "." + typ.Name
		if visiting[key] {
			return DefaultResolution{Kind: NoDefault}
		}
		visiting[key] = true
		defer delete(visiting, key)
		result := DefaultResolution{Kind: StructDefault}
		for _, field := range typ.Fields {
			if isEventType(field.Type) {
				continue
			}
			value := defaultValueOf(field.Type, visiting)
			if value.Kind == NoDefault {
				return DefaultResolution{Kind: NoDefault}
			}
			result.Fields = append(result.Fields, DefaultField{Name: field.Name, Value: value})
		}
		return result
	default:
		return DefaultResolution{Kind: NoDefault}
	}
}

func defaultConstantCompatible(typ Type, value DefaultConstant) bool {
	switch typ.Kind {
	case IntType, UintType:
		return value.Integer != nil
	case FloatType, DecimalType:
		return value.Kind == FloatType || value.Kind == DecimalType || value.Integer != nil
	case StringType:
		return value.Kind == StringType
	case BoolType:
		return value.Kind == BoolType
	case CharType, RuneType:
		return value.Integer != nil || value.Kind == typ.Kind || typ.Kind == RuneType && value.Kind == CharType
	default:
		return false
	}
}

func scalarDefault(typ Type, value DefaultConstant) DefaultResolution {
	if !defaultConstantSatisfies(typ, value) {
		return DefaultResolution{Kind: NoDefault}
	}
	return DefaultResolution{Kind: kindForNamed(typ, PrimitiveDefault), Value: value}
}

func kindForNamed(typ Type, fallback DefaultKind) DefaultKind {
	if typ.Named && fallback == PrimitiveDefault {
		return NamedDefault
	}
	return fallback
}

func integerDefault(typ Type) (*big.Int, DefaultKind, bool) {
	zero := big.NewInt(0)
	if integerSatisfiesContracts(typ, zero) {
		return zero, PrimitiveDefault, true
	}
	positive, positiveOK := nearestIntegerDefault(typ, true)
	negative, negativeOK := nearestIntegerDefault(typ, false)
	if positiveOK && negativeOK {
		comparison := new(big.Int).Abs(positive).Cmp(new(big.Int).Abs(negative))
		if comparison == 0 {
			return nil, NoDefault, false
		}
		if comparison < 0 {
			return positive, RangeDefault, true
		}
		return negative, RangeDefault, true
	}
	if positiveOK {
		return positive, RangeDefault, true
	}
	if negativeOK {
		return negative, RangeDefault, true
	}
	return nil, NoDefault, false
}

func decimalDefault(typ Type) DefaultResolution {
	zero := DefaultConstant{Kind: DecimalType, Lexeme: "0.0", Exact: new(big.Rat)}
	if defaultConstantSatisfies(typ, zero) {
		return scalarDefault(typ, zero)
	}

	var candidates []DefaultConstant
	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		if rangeContract.ExactMin != nil && rangeContract.ExactMin.Sign() > 0 {
			candidates = append(candidates, DefaultConstant{Kind: DecimalType, Lexeme: rangeContract.MinLexeme, Exact: new(big.Rat).Set(rangeContract.ExactMin)})
		}
		if rangeContract.ExactMax != nil && rangeContract.ExactMax.Sign() < 0 && !rangeContract.Exclusive {
			candidates = append(candidates, DefaultConstant{Kind: DecimalType, Lexeme: rangeContract.MaxLexeme, Exact: new(big.Rat).Set(rangeContract.ExactMax)})
		}
	}
	var best *DefaultConstant
	for i := range candidates {
		candidate := candidates[i]
		if !defaultConstantSatisfies(typ, candidate) {
			continue
		}
		if best == nil {
			best = &candidate
			continue
		}
		candidateAbs := new(big.Rat).Abs(candidate.Exact)
		bestAbs := new(big.Rat).Abs(best.Exact)
		switch candidateAbs.Cmp(bestAbs) {
		case -1:
			best = &candidate
		case 0:
			if candidate.Exact.Cmp(best.Exact) != 0 {
				return DefaultResolution{Kind: NoDefault}
			}
		}
	}
	if best == nil {
		return DefaultResolution{Kind: NoDefault}
	}
	return DefaultResolution{Kind: RangeDefault, Value: *best}
}

func nearestIntegerDefault(typ Type, positive bool) (*big.Int, bool) {
	magnitude := big.NewInt(1)
	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		if positive && rangeContract.Min != nil && rangeContract.Min.Sign() > 0 && rangeContract.Min.Cmp(magnitude) > 0 {
			magnitude.Set(rangeContract.Min)
		}
		if !positive && rangeContract.Max != nil && rangeContract.Max.Sign() < 0 {
			candidate := new(big.Int).Abs(rangeContract.Max)
			if rangeContract.Exclusive {
				candidate.Add(candidate, big.NewInt(1))
			}
			if candidate.Cmp(magnitude) > 0 {
				magnitude.Set(candidate)
			}
		}
	}
	step := big.NewInt(1)
	for _, contract := range typ.Contracts {
		if multiple, ok := contract.(MultipleOfContract); ok && multiple.Value != nil && multiple.Value.Sign() != 0 {
			factor := new(big.Int).Abs(multiple.Value)
			gcd := new(big.Int).GCD(nil, nil, step, factor)
			step.Mul(new(big.Int).Quo(step, gcd), factor)
		}
	}
	quotient, remainder := new(big.Int).QuoRem(magnitude, step, new(big.Int))
	if remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	magnitude.Mul(quotient, step)
	for _, contract := range typ.Contracts {
		marker, ok := contract.(MarkerContract)
		if !ok {
			continue
		}
		wantOdd := marker.Name == "odd"
		wantEven := marker.Name == "even"
		if !wantOdd && !wantEven {
			continue
		}
		matches := magnitude.Bit(0) == 1
		if wantEven {
			matches = !matches
		}
		if !matches {
			if step.Bit(0) == 0 {
				return nil, false
			}
			magnitude.Add(magnitude, step)
		}
	}
	candidate := new(big.Int).Set(magnitude)
	if !positive {
		candidate.Neg(candidate)
	}
	return candidate, integerSatisfiesContracts(typ, candidate)
}

func defaultConstantSatisfies(typ Type, value DefaultConstant) bool {
	for _, contract := range typ.Contracts {
		membership, ok := contract.(MembershipContract)
		if !ok {
			continue
		}
		member := false
		for _, allowed := range membership.Values {
			if defaultConstantsEqual(value, allowed) {
				member = true
				break
			}
		}
		if !member {
			return false
		}
	}
	if (typ.Kind == DecimalType || typ.Kind == FloatType) && value.Integer != nil {
		value.Exact = new(big.Rat).SetInt(value.Integer)
		value.Integer = nil
	}
	if value.Integer != nil {
		return integerSatisfiesContracts(typ, value.Integer)
	}
	if value.Exact != nil {
		for _, contract := range typ.Contracts {
			switch c := contract.(type) {
			case RangeContract:
				if c.ExactMin == nil && c.ExactMax == nil {
					return false
				}
				if c.ExactMin != nil && value.Exact.Cmp(c.ExactMin) < 0 {
					return false
				}
				if c.ExactMax != nil && (value.Exact.Cmp(c.ExactMax) > 0 || c.Exclusive && value.Exact.Cmp(c.ExactMax) == 0) {
					return false
				}
			case MarkerContract:
				if c.Name != "finite" {
					return false
				}
			case MultipleOfContract:
				return false
			}
		}
		return true
	}
	for _, contract := range typ.Contracts {
		switch c := contract.(type) {
		case MarkerContract:
			switch c.Name {
			case "finite":
			case "notEmpty":
				if value.Kind != StringType || value.String == "" {
					return false
				}
			default:
				return false
			}
		case RangeContract, MultipleOfContract:
			return false
		}
	}
	return true
}

func defaultConstantsEqual(left, right DefaultConstant) bool {
	leftExact := left.Exact
	if leftExact == nil && left.Integer != nil {
		leftExact = new(big.Rat).SetInt(left.Integer)
	}
	rightExact := right.Exact
	if rightExact == nil && right.Integer != nil {
		rightExact = new(big.Rat).SetInt(right.Integer)
	}
	if leftExact != nil || rightExact != nil {
		return leftExact != nil && rightExact != nil && leftExact.Cmp(rightExact) == 0
	}
	if left.Kind != right.Kind {
		return left.Kind == CharType && right.Kind == RuneType || left.Kind == RuneType && right.Kind == CharType
	}
	switch left.Kind {
	case StringType, CharType, RuneType:
		return left.String == right.String
	case BoolType:
		return left.Bool == right.Bool
	default:
		return left.Lexeme == right.Lexeme
	}
}

func integerSatisfiesContracts(typ Type, value *big.Int) bool {
	if typ.Kind == UintType && value.Sign() < 0 {
		return false
	}
	if typ.MinInteger != nil && value.Cmp(typ.MinInteger) < 0 {
		return false
	}
	if typ.MaxInteger != nil && value.Cmp(typ.MaxInteger) > 0 {
		return false
	}
	for _, contract := range typ.Contracts {
		switch c := contract.(type) {
		case RangeContract:
			if c.Min != nil && value.Cmp(c.Min) < 0 {
				return false
			}
			if c.Max != nil && (value.Cmp(c.Max) > 0 || c.Exclusive && value.Cmp(c.Max) == 0) {
				return false
			}
		case MultipleOfContract:
			if c.Value != nil && c.Value.Sign() != 0 && new(big.Int).Rem(new(big.Int).Abs(value), new(big.Int).Abs(c.Value)).Sign() != 0 {
				return false
			}
		case MarkerContract:
			switch c.Name {
			case "even":
				if value.Bit(0) != 0 {
					return false
				}
			case "odd":
				if value.Bit(0) == 0 {
					return false
				}
			}
		}
	}
	return true
}

func defaultConstantFromExpression(expr ast.Expression) (DefaultConstant, bool) {
	switch value := expr.(type) {
	case *ast.IntegerLiteral:
		integer, ok := ast.ParseIntegerLiteralLexeme(value.Token.Lexeme)
		if !ok {
			return DefaultConstant{}, false
		}
		return DefaultConstant{Kind: IntType, Lexeme: value.Token.Lexeme, Integer: integer}, true
	case *ast.PrefixExpression:
		integer, ok := constantIntegerValue(value)
		if !ok {
			return DefaultConstant{}, false
		}
		return DefaultConstant{Kind: IntType, Lexeme: integer.String(), Integer: integer}, true
	case *ast.InfixExpression:
		integer, ok := constantIntegerValue(value)
		if !ok {
			return DefaultConstant{}, false
		}
		return DefaultConstant{Kind: IntType, Lexeme: integer.String(), Integer: integer}, true
	case *ast.StringLiteral:
		return DefaultConstant{Kind: StringType, Lexeme: value.Token.Lexeme, String: value.Value}, true
	case *ast.FloatLiteral:
		exact, lexeme, ok := exactNumericConstant(value)
		if !ok {
			return DefaultConstant{}, false
		}
		return DefaultConstant{Kind: DecimalType, Lexeme: lexeme, Exact: exact}, true
	case *ast.CharLiteral:
		return DefaultConstant{Kind: CharType, Lexeme: value.Token.Lexeme, String: value.Value}, true
	case *ast.BooleanLiteral:
		return DefaultConstant{Kind: BoolType, Lexeme: value.Token.Lexeme, Bool: value.Value}, true
	default:
		return DefaultConstant{}, false
	}
}

func exactNumericConstant(expr ast.Expression) (*big.Rat, string, bool) {
	if expr == nil {
		return nil, "", false
	}
	switch value := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		lexeme, suffix := ast.SplitNumericLiteralSuffix(value.TokenLiteral())
		if suffix == "t" || suffix == "r" {
			return nil, "", false
		}
		if integer, ok := ast.ParseIntegerFormNumericLiteralLexeme(value.TokenLiteral()); ok {
			return new(big.Rat).SetInt(integer), value.TokenLiteral(), true
		}
		lexeme = ast.NormalizeNumericLiteralLexeme(lexeme)
		exact, ok := new(big.Rat).SetString(lexeme)
		return exact, value.TokenLiteral(), ok
	case *ast.PrefixExpression:
		if value.Operator == "+" {
			return exactNumericConstant(value.Right)
		}
		if value.Operator != "-" {
			return nil, "", false
		}
		exact, lexeme, ok := exactNumericConstant(value.Right)
		if !ok {
			return nil, "", false
		}
		return exact.Neg(exact), "-" + lexeme, true
	default:
		return nil, "", false
	}
}

func defaultExpression(resolution DefaultResolution, typ Type, token lexer.Token) ast.Expression {
	switch resolution.Kind {
	case PrimitiveDefault, NamedDefault, RangeDefault, MembershipDefault, ExplicitTypeDefault:
		return defaultConstantExpression(resolution.Value, token)
	case EnumDefault:
		return &ast.MemberExpression{
			Token:    token,
			Object:   &ast.Identifier{Token: token, Value: typ.Name},
			Property: &ast.Identifier{Token: token, Value: resolution.Value.Lexeme},
		}
	case UnionDefault:
		variant, ok := unionVariantByName(typ, resolution.Variant)
		if !ok {
			return nil
		}
		member := &ast.MemberExpression{Token: token, Object: &ast.Identifier{Token: token, Value: typ.Name}, Property: &ast.Identifier{Token: token, Value: resolution.Variant}}
		if resolution.Payload != nil && variant.Payload != nil {
			return &ast.CallExpression{Token: token, Callee: member, Arguments: []ast.Expression{defaultExpression(*resolution.Payload, *variant.Payload, token)}}
		}
		if len(variant.PayloadFields) > 0 {
			literal := &ast.StructLiteral{Token: token, Type: &ast.TypeReference{Token: token, Name: typ.Name + "." + resolution.Variant}}
			for _, field := range resolution.Fields {
				fieldType, ok := unionPayloadFieldType(variant, field.Name)
				if ok {
					literal.Fields = append(literal.Fields, &ast.StructLiteralField{Token: token, Name: &ast.Identifier{Token: token, Value: field.Name}, Value: defaultExpression(field.Value, fieldType, token)})
				}
			}
			return literal
		}
		return member
	case StructDefault:
		literal := &ast.StructLiteral{Token: token, Type: &ast.TypeReference{Token: token, Name: typ.Name}}
		for i, field := range resolution.Fields {
			fieldType, ok := lookupStructField(typ, field.Name)
			if !ok {
				continue
			}
			literal.Fields = append(literal.Fields, &ast.StructLiteralField{Token: token, Name: &ast.Identifier{Token: token, Value: field.Name}, Value: defaultExpression(field.Value, fieldType, token)})
			_ = i
		}
		return literal
	case ArrayDefault:
		literal := &ast.ArrayLiteral{Token: token}
		if typ.Element != nil {
			for _, element := range resolution.Elements {
				literal.Elements = append(literal.Elements, defaultExpression(element, *typ.Element, token))
			}
		}
		return literal
	default:
		return nil
	}
}

func defaultConstantExpression(value DefaultConstant, token lexer.Token) ast.Expression {
	token.Lexeme = value.Lexeme
	switch value.Kind {
	case BoolType:
		return &ast.BooleanLiteral{Token: token, Value: value.Bool}
	case StringType:
		return &ast.StringLiteral{Token: token, Value: value.String}
	case FloatType, DecimalType:
		return &ast.FloatLiteral{Token: token}
	case CharType:
		if value.Integer == nil {
			return &ast.CharLiteral{Token: token, Value: value.String}
		}
	}
	if value.Integer != nil && value.Integer.Sign() < 0 {
		absolute := new(big.Int).Abs(value.Integer)
		innerToken := token
		innerToken.Lexeme = absolute.String()
		return &ast.PrefixExpression{Token: token, Operator: "-", Right: &ast.IntegerLiteral{Token: innerToken, Value: absolute.Int64(), BigValue: absolute}}
	}
	integer := int64(0)
	if value.Integer != nil && value.Integer.IsInt64() {
		integer = value.Integer.Int64()
	}
	return &ast.IntegerLiteral{Token: token, Value: integer, BigValue: value.Integer}
}
