package sema

import "sec/internal/ast"

// resolvedPropertyAccessKind classifies one validated property operation.
// Compound assignment retains its read-modify-write identity even when its
// setter is fallible; that error edge is carried separately on the fact.
//
// Rules:
//   - rules/declarations/properties.md — §§4–7 and §15
//   - rules/corrections/applied/semantic-ir-properties-correction-20260813.md
func resolvedPropertyAccessKind(property Property, operator string) ResolvedPropertyAccessKind {
	if operator != "" && operator != "=" {
		return PropertyCompoundUpdate
	}
	if property.Fallible {
		return PropertyFallibleWrite
	}
	return PropertyWrite
}

// recordResolvedPropertyRead records a getter operation after readability has
// been established. It is intentionally separate from raw metadata lookup so
// write-only properties never acquire a false read fact.
//
// Rules:
//   - rules/declarations/properties.md — §4 "Getter semantics" and §15 "Lowering"
func (a *Analyzer) recordResolvedPropertyRead(expr ast.Expression, owner Type, property Property) {
	a.recordResolvedPropertyAccess(expr, owner, property, PropertyRead, "")
}

// recordResolvedPropertyWrite records the setter operation selected for a
// successful simple, fallible, or compound property assignment.
//
// Rules:
//   - rules/declarations/properties.md — §§5–7 and §15
func (a *Analyzer) recordResolvedPropertyWrite(expr ast.Expression, owner Type, property Property, operator string) {
	a.recordResolvedPropertyAccess(expr, owner, property, resolvedPropertyAccessKind(property, operator), operator)
}

// recordResolvedPropertyAccess stores one compiler-owned access fact for
// Semantic IR, lowering, and tooling consumers.
//
// Rules:
//   - rules/declarations/properties.md — §15 "Lowering"
//   - rules/corrections/applied/semantic-ir-properties-correction-20260813.md
func (a *Analyzer) recordResolvedPropertyAccess(expr ast.Expression, owner Type, property Property, kind ResolvedPropertyAccessKind, operator string) {
	if a == nil || expr == nil {
		return
	}
	if a.resolvedPropertyAccesses == nil {
		a.resolvedPropertyAccesses = map[ast.Expression]ResolvedPropertyAccess{}
	}
	a.resolvedPropertyAccesses[expr] = ResolvedPropertyAccess{
		Kind:         kind,
		OwnerType:    owner,
		PropertyType: property.Type,
		Name:         property.Name,
		Static:       property.Static,
		Fallible:     property.Fallible,
		Operator:     operator,
	}
}

// propertyOwnerForMember resolves the nominal owner used by an already
// selected instance or type-qualified static property operation.
//
// Rules:
//   - rules/declarations/properties.md — §§8–9 "Instance receiver" and "Static properties"
func (a *Analyzer) propertyOwnerForMember(expr *ast.MemberExpression, static bool) Type {
	if static {
		if path, ok := typePathFromExpression(expr.Object); ok {
			if owner, exists := a.types[a.resolveTypeName(path)]; exists {
				return owner
			}
		}
		return Type{}
	}
	owner, _ := a.inferPlaceBase(expr.Object)
	return dereferenceType(owner)
}
