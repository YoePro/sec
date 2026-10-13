package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// lowerPayloadBorrow lowers a `ref` or `ref mut` union payload binding of a
// match arm from Sema's resolved reference plan for the pattern's ref node.
// The payload Place is derived from the match subject's own Place root, not
// from a copy of the union or of the payload: the root is formed as for any
// local borrow, place.union-payload names the arm's union.is-variant
// predicate as its active-variant proof, and the borrow carries the plan's
// facts unchanged. The binding stays arm-scoped; Sema has already rejected
// escapes of a match-scoped payload origin.
//
// Supported: a single-payload variant of a union or Option whose subject is a
// whole local binding or by-value parameter. Result payloads (Result is not a
// Semantic IR union), struct-like payloads, subjects reached through fields,
// elements or references, and reborrows stay package-15 unsupported.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 19, 44, 71, 99, 100, 121
//   - rules/memory/borrowing.md — § 12(5)
func (fb *functionBuilder) lowerPayloadBorrow(subjectExpr ast.Expression, subject builtValue, arm *ast.MatchArm, armPlan sema.ResolvedMatchArm, guard ValueID) (builtValue, bool, error) {
	token := arm.Token
	reference := matchArmPayloadReference(arm.Pattern.Expression())
	if reference == nil {
		return builtValue{}, false, fb.unsupported("borrowed match payload without a ref pattern", token)
	}
	token = reference.Token
	if armPlan.PatternKind != sema.MatchPatternUnionVariant && armPlan.PatternKind != sema.MatchPatternOptionSome {
		return builtValue{}, false, fb.unsupported("borrowed "+string(armPlan.PatternKind)+" payload (Result payload Places are not Semantic IR union Places)", token)
	}
	plan, ok := fb.owner.analyzer.ResolvedReferencePlanOf(reference.Value)
	if !ok {
		return builtValue{}, false, fb.unsupported("borrowed match payload without a resolved reference plan", token)
	}
	if !plan.Facts.Checkable() {
		return builtValue{}, false, fb.unsupported("uncheckable borrowed match payload", token)
	}
	if plan.Reborrow || plan.Implicit {
		return builtValue{}, false, fb.unsupported("borrowed match payload of a subject reached through a reference", token)
	}
	projections := plan.Place.Projections
	if len(projections) != 1 || projections[0].Canonical() != sema.CanonicalProjectionUnionPayload {
		return builtValue{}, false, fb.unsupported("borrowed match payload of a subject that is not a whole local binding", token)
	}
	if guard == 0 {
		return builtValue{}, false, fb.unsupported("borrowed match payload without an active-variant test", token)
	}
	definition, ok := fb.owner.unionDefinition(subject.typ)
	if !ok || int(armPlan.UnionVariantIndex) >= len(definition.Variants) {
		return builtValue{}, false, fb.unsupported("unresolved union match payload", token)
	}
	variant := definition.Variants[armPlan.UnionVariantIndex]
	if variant.Kind != UnionVariantSingle || definition.Variants[armPlan.UnionVariantIndex].Name != projections[0].Name {
		return builtValue{}, false, fb.unsupported("borrowed struct-like or mismatched union payload", token)
	}
	root, err := fb.lowerRootPlace(reference, plan, subjectExpr)
	if err != nil {
		return builtValue{}, false, err
	}
	rootType, ok := fb.owner.module.Types.Lookup(root.typ)
	if !ok || rootType.Element != subject.typ {
		return builtValue{}, false, fb.unsupported("borrowed match payload whose root is not the match subject", token)
	}
	if reference.Mutable && !rootType.Mutable {
		return builtValue{}, false, fb.unsupported("mutable borrow of a read-only union payload", token)
	}
	facts := placeMetadataOf(plan.Place)
	facts.VariantProven = true
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: variant.Payload, Mutable: rootType.Mutable})
	payload := fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{root.id}, UnionVariant: UnionVariantIndex(armPlan.UnionVariantIndex),
		VariantGuard: guard, Place: facts, Location: location(reference.Token),
	}, placeType)
	borrowed, err := fb.emitBorrow(reference, plan, payload)
	if err != nil {
		return builtValue{}, false, err
	}
	return borrowed, true, nil
}

// matchArmPayloadReference returns the ref or ref mut node binding a payload
// in a variant pattern.
func matchArmPayloadReference(pattern ast.Expression) *ast.RefExpression {
	var argument ast.Expression
	switch pattern := pattern.(type) {
	case *ast.CallExpression:
		if len(pattern.Arguments) == 1 {
			argument = pattern.Arguments[0]
		}
	case *ast.OkExpression:
		argument = pattern.Value
	case *ast.ErrExpression:
		argument = pattern.Value
	}
	reference, _ := argument.(*ast.RefExpression)
	return reference
}
