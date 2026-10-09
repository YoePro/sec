package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema/membership"
)

// payloadlessUnion identifies the union family whose complete semantic domain
// has compile-time equality without evaluating payloads or comparing storage.
// Rules: rules/declarations/unions.md — §14 Equality;
// rules/types/contracts.md — Applicability and Ordered membership.
func payloadlessUnion(typ Type) bool {
	if typ.Kind != UnionType || len(typ.UnionVariants) == 0 {
		return false
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			return false
		}
	}
	return true
}

// nominalConstantOwner retains the declaring identity and concrete generic
// arguments even when a value is spelled through several named derivations.
// Rules: rules/types/types.md — Named types; rules/declarations/unions.md — §14;
// rules/types/contracts.md — Ordered membership.
func nominalConstantOwner(typ Type) string {
	if typ.ConstantBaseName != "" {
		typ.Name = typ.ConstantBaseName
		typ.FixedNamedArguments = false
	}
	return typeDisplayName(typ)
}

// nominalMemberOwner resolves a type-qualified constructor without executing a
// getter. The current declaration supplies its own identity during contracts
// and explicit-default analysis, before it is published in the type registry.
// Rules: rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext;
// rules/declarations/unions.md — §5 Construction; rules/types/types.md — Named types.
func (a *Analyzer) nominalMemberOwner(member *ast.MemberExpression, context Type) (Type, bool) {
	if member.Property == nil {
		return Type{}, false
	}
	object := member.Object
	if index, ok := object.(*ast.IndexExpression); ok && len(member.OwnerGenericArguments) > 0 {
		object = index.Left
	}
	name, ok := typePathFromExpression(object)
	if !ok {
		return Type{}, false
	}
	name = a.resolveTypeName(name)
	if (context.Kind == EnumType || context.Kind == UnionType) && name == context.Name && len(member.OwnerGenericArguments) == 0 {
		return context, true
	}
	typ, ok := a.types[name]
	if !ok || typ.Kind != EnumType && typ.Kind != UnionType || !a.canAccessDeclaredName(typ.Name, typ.Module) {
		return Type{}, false
	}
	if len(member.OwnerGenericArguments) > 0 {
		typ, ok = a.resolveType(&ast.TypeReference{Token: member.Token, Name: name, TypeArgs: member.OwnerGenericArguments})
		if !ok {
			return Type{}, false
		}
	} else if len(typ.GenericParameters) > 0 {
		return Type{}, false
	}
	return typ, true
}

// nominalMemberConstant establishes enum values and payload-less union values
// in the shared semantic CTE model. Identity includes the declared type and
// member, so unrelated types with same-spelled members never compare equal.
// Rules: rules/types/contracts.md — Ordered membership;
// rules/declarations/unions.md — §§5,14; rules/compiler/compile_time_evaluation.md — §4.
func (a *Analyzer) nominalMemberConstant(expr ast.Expression, context Type) (DefaultConstant, bool) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok {
		return DefaultConstant{}, false
	}
	typ, ok := a.nominalMemberOwner(member, context)
	if !ok {
		return DefaultConstant{}, false
	}
	name := member.Property.Value
	key := "variant:" + name
	if typ.Kind == EnumType {
		if !enumHasMember(typ, name) {
			return DefaultConstant{}, false
		}
		if value, found := typ.EnumConsts[name]; found {
			key, ok = enumValueClassKey(value)
			if !ok {
				return DefaultConstant{}, false
			}
		}
	} else {
		variant, found := unionVariantByName(typ, name)
		if !payloadlessUnion(typ) || !found || variant.Payload != nil || len(variant.PayloadFields) > 0 {
			return DefaultConstant{}, false
		}
	}
	owner := nominalConstantOwner(typ)
	sourceType := typeDisplayName(typ)
	if typ.ConstantBaseName != "" && len(typ.GenericParameters) == 0 {
		sourceType = typ.Name
	}
	return DefaultConstant{Kind: typ.Kind, Nominal: membership.Value{Owner: owner, Member: name, Key: key, Type: sourceType, TypeName: typ.Name}, Lexeme: owner + "." + name, String: name}, true
}

// nominalConstantCompatible accepts the current declaration and its named
// ancestry, with the exact concrete base identity. Same-spelled variants in
// sibling derivations or distinct generic instances introduce no coercion.
// Rules: rules/types/contracts.md — Ordered membership (compatible base type,
// semantic equality, no coercion); rules/types/types.md — Named types.
func nominalConstantCompatible(typ Type, value membership.Value) bool {
	if value.Owner != nominalConstantOwner(typ) {
		return false
	}
	if value.TypeName == typ.Name {
		return true
	}
	for _, ancestor := range typ.ConstantAncestors {
		if value.TypeName == ancestor {
			return true
		}
	}
	return false
}

// enumHasMember validates a member against its resolved enum declaration.
// Rules: rules/declarations/enums.md — Enum values.
func enumHasMember(typ Type, name string) bool {
	for _, value := range typ.EnumValues {
		if value == name {
			return true
		}
	}
	return false
}

// checkNominalMembershipValue proves a known enum/union value at a storage,
// return, argument or explicit-conversion boundary, including immutable CTE
// dependencies. Runtime-dependent values remain for runtime contract validation.
// Rules: rules/types/contracts.md — Initialization and assignment, Ordered membership.
func (a *Analyzer) checkNominalMembershipValue(typ Type, expr ast.Expression) bool {
	if typ.Kind != EnumType && typ.Kind != UnionType || !hasContracts(typ) {
		return false
	}
	value, outcome := a.semanticCompileTimeConstantVisiting(expr, map[string]bool{}, typ)
	if outcome != compileTimeEvaluated || !defaultConstantCompatible(typ, value) {
		return false
	}
	if violated, ok := firstViolatedContract(typ, value); ok {
		a.addContractError(expressionToken(expr), violated, diagnostics.ValueViolatesContract,
			"use a value satisfying every contract of the named type", "%s value %s violates in contract %s", typ.Kind, value.Lexeme, typ.Name)
		return true
	}
	return false
}
