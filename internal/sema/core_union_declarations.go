package sema

import "sec/internal/ast"

// exactCoreUnionDeclaration validates a closed compiler-owned union template
// before refining its declaration/variant source identities. Owning payloads
// cannot be replaced by references, other types, extra variants or constraints.
// Rules: rules/library/core-library.md — compiler-known core declarations;
// rules/concurrency/threads.md — §4; rules/concurrency/channels.md — §14.
func (a *Analyzer) exactCoreUnionDeclaration(stmt *ast.TypeDeclStatement, typ Type, rulebook string) Type {
	valid := stmt.Union && stmt.ErrorType == typ.ErrorAssignable && stmt.BaseType == nil && stmt.AssignedType == nil && stmt.StructType == nil && stmt.RegisterType == nil && len(stmt.Variants) == 0 && len(stmt.Attributes) == 0 && stmt.Contract == nil && stmt.Default == nil && len(stmt.Implements) == 0 && len(stmt.GenericParameters) == len(typ.GenericParameters) && len(stmt.UnionVariants) == len(typ.UnionVariants)
	if valid {
		for i, parameter := range stmt.GenericParameters {
			if parameter == nil || parameter.Name == nil || parameter.Name.Value != typ.GenericParameters[i] || parameter.TypePack || len(parameter.Constraints) != 0 {
				valid = false
				break
			}
		}
	}
	if valid {
		for i, variant := range stmt.UnionVariants {
			expected := typ.UnionVariants[i]
			if variant == nil || variant.Name == nil || variant.Name.Value != expected.Name || variant.Default != expected.Default || len(variant.PayloadFields) != 0 || (variant.Payload != nil) != (expected.Payload != nil) {
				valid = false
				break
			}
			if variant.Payload != nil && (variant.Payload.Invalid || variant.Payload.Ref || variant.Payload.MutableRef || variant.Payload.Name != expected.Payload.Name || typeReferenceDisplayName(variant.Payload) != expected.Payload.Name) {
				valid = false
				break
			}
		}
	}
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "%s requires its exact canonical core union declaration from %s", typ.Name, rulebook)
		return typ
	}
	typ.DeclarationToken = stmt.Name.Token
	for i, variant := range stmt.UnionVariants {
		typ.UnionVariants[i].Token = variant.Name.Token
		a.recordDefinition(variant.Name.Token)
	}
	return typ
}
