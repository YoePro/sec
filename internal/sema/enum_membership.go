package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// enumMemberConstant resolves `Enum.Member` (through the enum's own name or a
// named type over it) to a compile-time membership constant. Enum values
// support compile-time semantic equality by member identity, so they are
// valid `in [...]` members of an enum-based named type.
//
// Rules:
//   - rules/types/contracts.md — "Applicability" (`in [...]` on named types whose values support compile-time equality)
//   - rules/types/contracts.md — "Ordered membership" (semantic equality, no memory identity or coercion)
func enumMemberConstant(typ Type, expr ast.Expression) (DefaultConstant, bool) {
	if typ.Kind != EnumType {
		return DefaultConstant{}, false
	}
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return DefaultConstant{}, false
	}
	owner, ok := member.Object.(*ast.Identifier)
	if !ok || owner.Value != typ.Name && owner.Value != enumBaseName(typ) {
		return DefaultConstant{}, false
	}
	if !enumHasMember(typ, member.Property.Value) {
		return DefaultConstant{}, false
	}
	return DefaultConstant{Kind: EnumType, Lexeme: enumBaseName(typ) + "." + member.Property.Value, String: member.Property.Value}, true
}

// enumBaseName returns the declared enum identity underlying typ.
func enumBaseName(typ Type) string {
	if typ.Named && typ.Underlying != "" {
		return typ.Underlying
	}
	return typ.Name
}

func enumHasMember(typ Type, name string) bool {
	for _, value := range typ.EnumValues {
		if value == name {
			return true
		}
	}
	return false
}

// checkEnumMembershipValue proves a compile-time enum member against the
// membership contracts of an enum-based named type.
//
// Rules:
//   - rules/types/contracts.md — "Initialization and assignment"
//   - rules/types/contracts.md — "Ordered membership"
func (a *Analyzer) checkEnumMembershipValue(typ Type, expr ast.Expression) bool {
	constant, ok := enumMemberConstant(typ, expr)
	if !ok {
		return false
	}
	for _, contract := range typ.Contracts {
		membership, ok := contract.(MembershipContract)
		if !ok {
			continue
		}
		member := false
		for _, allowed := range membership.Values {
			if defaultConstantsEqual(constant, allowed) {
				member = true
				break
			}
		}
		if !member {
			a.addErrorAtTokenWithMetadata(expressionToken(expr), diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "enum value %s violates in contract %s", constant.String, typ.Name)
			return true
		}
	}
	return false
}
