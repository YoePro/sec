package sema

import "sec/internal/ast"

// sumSourceKind retains the source declaration category in construction errors.
// Rules: rules/declarations/unions.md — Construction; rules/concurrency/atomics.md — §6.
func sumSourceKind(typ Type) string {
	if typ.AssociatedEnum {
		return "enum"
	}
	return "union"
}

// inferCallAsUnionVariantConstructor resolves the generic owner and payload,
// preserving source enum identity for canonical associated-value core enums.
// Rules: rules/declarations/unions.md — Construction; rules/declarations/generics.md — Generic unions;
// rules/concurrency/atomics.md — §§6(1–8),17(3–5).
func (a *Analyzer) inferCallAsUnionVariantConstructor(expr *ast.CallExpression, expected *Type) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok {
		return Type{}, expressionValue{}, false
	}

	typeName, ok := typePathFromExpression(member.Object)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	typeName = a.resolveTypeName(typeName)

	template, ok := a.types[typeName]
	if !ok || template.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}
	if definition, exists := a.typeDefinitionTokens[typeName]; exists {
		a.bindDefinition(expressionToken(member.Object), definition)
	}

	variant, ok := lookupUnionVariant(template, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown %s variant %s.%s", sumSourceKind(template), template.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	a.bindDefinition(member.Property.Token, variant.Token)

	unionType := template
	if len(expr.GenericArguments) > 0 {
		explicit, ok := a.resolveType(&ast.TypeReference{
			Token:    expr.Token,
			Name:     typeName,
			TypeArgs: expr.GenericArguments,
		})
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = explicit
	} else if expected != nil && expected.Kind == UnionType && expected.Name == template.Name {
		unionType = *expected
	} else if len(template.GenericParameters) > 0 {
		// Generic inference observes the same owning-payload context as the
		// subsequent concrete payload check, including explicit <- syntax.
		a.constructionOwnershipDepth++
		concrete, ok := a.inferGenericUnionVariantInstance(template, variant, expr)
		a.constructionOwnershipDepth--
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		unionType = concrete
	}

	concreteVariant, ok := lookupUnionVariant(unionType, member.Property.Value)
	if !ok {
		a.addErrorAtToken(member.Property.Token, "unknown union variant %s.%s", unionType.Name, member.Property.Value)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if len(concreteVariant.PayloadFields) > 0 {
		a.addErrorAtToken(expr.Token, "%s variant %s.%s requires named payload fields", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if concreteVariant.Payload == nil {
		if unionType.AssociatedEnum {
			a.addErrorAtToken(member.Property.Token, "%s.%s is a payload-less enum value; omit parentheses", typeDisplayName(unionType), concreteVariant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "%s variant %s.%s expects 0 arguments, got %d", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		return unionType, expressionValue{Display: expr.String()}, true
	}

	if len(expr.Arguments) != 1 {
		a.addErrorAtToken(expr.Token, "%s variant %s.%s expects 1 argument, got %d", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	payloadType := *concreteVariant.Payload
	valueType, _ := a.inferOwningConstructionValue(expr.Arguments[0], payloadType)
	if valueType.Kind != InvalidType && !a.canInitialize(payloadType, valueType, expr.Arguments[0]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "%s variant %s.%s payload must be %s, got %s", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name, typeDisplayName(payloadType), typeDisplayName(valueType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if valueType.Kind != InvalidType && a.checkCompileTimeContractExpression(payloadType, expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if valueType.Kind != InvalidType && !a.validateOwningConstructionSource(expr.Arguments[0]) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	return unionType, expressionValue{Display: expr.String()}, true
}
