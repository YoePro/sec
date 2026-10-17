package sema

import "sec/internal/ast"

// inferUnionVariantExpression resolves a payload-less sum variant against its
// concrete generic owner, retaining nominal identity and declaration navigation.
// The same machinery represents the canonical associated-value core enum.
// Rules: rules/declarations/unions.md — Construction; rules/declarations/generics.md — Generic unions;
// rules/concurrency/atomics.md — §§6(1–4),43–45.
func (a *Analyzer) inferUnionVariantExpression(expr *ast.MemberExpression) (Type, bool) {
	owner := expr.Object
	if len(expr.OwnerGenericArguments) > 0 {
		if indexed, ok := owner.(*ast.IndexExpression); ok {
			owner = indexed.Left
		}
	}
	typeName, ok := typePathFromExpression(owner)
	if !ok {
		return Type{}, false
	}
	typeName = a.resolveTypeName(typeName)

	typ, ok := a.types[typeName]
	if !ok || typ.Kind != UnionType {
		return Type{}, false
	}
	if definition, exists := a.typeDefinitionTokens[typeName]; exists {
		a.bindDefinition(expressionToken(owner), definition)
	}
	if len(expr.OwnerGenericArguments) > 0 {
		concrete, resolved := a.resolveType(&ast.TypeReference{Token: expr.Token, Name: typeName, TypeArgs: expr.OwnerGenericArguments})
		if !resolved {
			return Type{Kind: InvalidType}, true
		}
		typ = concrete
	}
	if len(typ.GenericParameters) > 0 && a.currentFunctionReturn.Kind == UnionType && a.currentFunctionReturn.Name == typ.Name {
		typ = a.currentFunctionReturn
	}
	if typ.AssociatedEnum && len(typ.GenericParameters) > 0 {
		a.addErrorAtToken(expr.Token, "%s requires %d generic arguments, got 0", typeName, len(typ.GenericParameters))
		return Type{Kind: InvalidType}, true
	}
	for _, variant := range typ.UnionVariants {
		if variant.Name != expr.Property.Value {
			continue
		}
		if variant.Payload != nil {
			a.bindDefinition(expr.Property.Token, variant.Token)
			a.reportUnionVariantPayloadRequired(expr.Property.Token, typ, variant)
			return Type{Kind: InvalidType}, true
		}
		a.bindDefinition(expr.Property.Token, variant.Token)
		return typ, true
	}
	a.addErrorAtToken(expr.Property.Token, "unknown %s variant %s.%s", sumSourceKind(typ), typ.Name, expr.Property.Value)
	return Type{Kind: InvalidType}, true
}
