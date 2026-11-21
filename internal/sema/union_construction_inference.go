package sema

import (
	"sec/internal/ast"
	"strings"
)

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
	delete(a.resolvedUnionConstructions, expr)
	errorsBefore := len(a.errors)
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
	ownerArguments := member.OwnerGenericArguments
	if len(ownerArguments) == 0 {
		ownerArguments = expr.GenericArguments
	}
	if len(ownerArguments) > 0 {
		explicit, ok := a.resolveType(&ast.TypeReference{
			Token:    expr.Token,
			Name:     typeName,
			TypeArgs: ownerArguments,
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
		a.addTaskCategoryError(expr.Token, unionType, "%s variant %s.%s requires named payload fields", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if concreteVariant.Payload == nil {
		if unionType.AssociatedEnum {
			a.addErrorAtToken(member.Property.Token, "%s.%s is a payload-less enum value; omit parentheses", typeDisplayName(unionType), concreteVariant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addTaskCategoryError(expr.Token, unionType, "%s variant %s.%s expects 0 arguments, got %d", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordUnionConstruction(expr, unionType, concreteVariant)
		return unionType, expressionValue{Display: expr.String()}, true
	}

	if len(expr.Arguments) != 1 {
		a.addTaskCategoryError(expr.Token, unionType, "%s variant %s.%s expects 1 argument, got %d", sumSourceKind(unionType), typeDisplayName(unionType), concreteVariant.Name, len(expr.Arguments))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if !a.checkUnionPayloadOperand(unionType, concreteVariant, "", expr.Arguments[0], *concreteVariant.Payload) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(a.errors) == errorsBefore {
		a.recordUnionConstruction(expr, unionType, concreteVariant)
	}
	return unionType, expressionValue{Display: expr.String()}, true
}

// inferExpectedUnionVariantExpression resolves contextual constructors against
// their expected concrete union and records only accepted payload preparation.
// Rules: rules/declarations/unions.md §§4–5,7.8;
// rules/compiler/semantic_ir.md §23.
func (a *Analyzer) inferExpectedUnionVariantExpression(expr ast.Expression, expected Type) (Type, expressionValue, bool) {
	delete(a.resolvedUnionConstructions, expr)
	errorsBefore := len(a.errors)
	if expected.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		variant, ok := lookupUnionVariant(expected, expr.Value)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.reportUnionVariantPayloadRequired(expr.Token, expected, variant)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(a.errors) == errorsBefore {
			a.recordUnionConstruction(expr, expected, variant)
		}
		return expected, expressionValue{Display: expr.String()}, true
	case *ast.MemberExpression:
		// rules/declarations/unions.md and rules/types/types.md: an expected
		// generic union type supplies the missing type arguments for a
		// qualified payload-less constructor such as Option.None. Resolve the
		// qualifier here instead of first producing the open Option type.
		owner, ok := typePathFromExpression(expr.Object)
		if !ok || a.resolveTypeName(owner) != expected.Name {
			return Type{}, expressionValue{}, false
		}
		variant, ok := lookupUnionVariant(expected, expr.Property.Value)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.reportUnionVariantPayloadRequired(expr.Property.Token, expected, variant)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.bindDefinition(expr.Property.Token, variant.Token)
		if len(a.errors) == errorsBefore {
			a.recordUnionConstruction(expr, expected, variant)
		}
		return expected, expressionValue{Display: expr.String()}, true
	case *ast.CallExpression:
		name := callExpressionName(expr)
		if name == "" {
			return Type{}, expressionValue{}, false
		}
		variant, ok := lookupUnionVariant(expected, name)
		if !ok {
			return Type{}, expressionValue{}, false
		}
		if len(variant.PayloadFields) > 0 {
			a.addTaskCategoryError(expr.Token, expected, "union variant %s.%s requires named payload fields", typeDisplayName(expected), variant.Name)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if variant.Payload == nil {
			if len(expr.Arguments) != 0 {
				a.addTaskCategoryError(expr.Token, expected, "union variant %s.%s expects 0 arguments, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
				return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
			}
			if len(a.errors) == errorsBefore {
				a.recordUnionConstruction(expr, expected, variant)
			}
			return expected, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addTaskCategoryError(expr.Token, expected, "union variant %s.%s expects 1 argument, got %d", typeDisplayName(expected), variant.Name, len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.checkUnionPayloadOperand(expected, variant, "", expr.Arguments[0], *variant.Payload) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(a.errors) == errorsBefore {
			a.recordUnionConstruction(expr, expected, variant)
		}
		return expected, expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}

// inferStructLiteralAsUnionVariant validates named payload fields and defaults
// before recording their declaration positions and source-order preparations.
// Rules: rules/declarations/unions.md §§5.3–5.4,7.8;
// rules/compiler/semantic_ir.md §23.
func (a *Analyzer) inferStructLiteralAsUnionVariant(expr *ast.StructLiteral) (Type, expressionValue, bool) {
	delete(a.resolvedUnionConstructions, expr)
	errorsBefore := len(a.errors)
	if expr.Type == nil || !strings.Contains(expr.Type.Name, ".") {
		return Type{}, expressionValue{}, false
	}

	unionName, variantName, ok := splitUnionVariantTypeName(expr.Type.Name)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	unionName = a.resolveTypeName(unionName)
	unionType, ok := a.synthesizedStructTypes[expr]
	if !ok {
		unionType, ok = a.types[unionName]
	}
	if !ok || unionType.Kind != UnionType {
		return Type{}, expressionValue{}, false
	}

	variant, ok := lookupUnionVariant(unionType, variantName)
	if !ok {
		a.addErrorAtToken(expr.Token, "unknown union variant %s.%s", unionType.Name, variantName)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(variant.PayloadFields) == 0 {
		a.addErrorAtToken(expr.Token, "union variant %s.%s requires unnamed payload construction", typeDisplayName(unionType), variant.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	if !a.checkUnionPayloadFields(unionType, variant, expr, expr.Token) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if len(a.errors) == errorsBefore {
		a.recordUnionConstruction(expr, unionType, variant)
	}
	return unionType, expressionValue{Display: expr.String()}, true
}
