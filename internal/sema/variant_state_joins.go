package sema

import (
	"sort"

	"sec/internal/ast"
)

// knownVariantSet maps a binding to the active variant (union variant,
// Some/None or Ok/Err) proven at one program point.
type knownVariantSet map[string]string

// currentKnownVariants collects every binding whose active variant is proven
// at this point by the current path facts (knownVariantTest), so the proof
// can cross a control-flow join.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 5 "Reachable-state exhaustiveness (MD-042)"
//   - rules/declarations/unions.md — § 8.1 "Active variant test"
func (a *Analyzer) currentKnownVariants() knownVariantSet {
	out := knownVariantSet{}
	seen := map[string]bool{}
	for _, active := range a.activeConditionFacts {
		if active.fact.Condition == nil {
			continue
		}
		for _, test := range conjunctStateTests(active.fact.Condition) {
			identifier, ok := test.Subject.(*ast.Identifier)
			if !ok || identifier == nil || seen[identifier.Value] {
				continue
			}
			seen[identifier.Value] = true
			symbol, exists := a.symbols.lookup(identifier.Value)
			if !exists {
				continue
			}
			for _, variant := range provableMatchVariants(symbol.Type) {
				if value, known := a.knownVariantTest(identifier.Value, variant); known && value {
					out[identifier.Value] = variant
					break
				}
			}
		}
	}
	return out
}

// intersectKnownVariants keeps the bindings proven to the same variant on
// every continuing edge of a join. No edge proves nothing.
func intersectKnownVariants(edges []knownVariantSet) knownVariantSet {
	if len(edges) == 0 {
		return nil
	}
	out := knownVariantSet{}
	for binding, variant := range edges[0] {
		common := true
		for _, edge := range edges[1:] {
			if edge[binding] != variant {
				common = false
				break
			}
		}
		if common {
			out[binding] = variant
		}
	}
	return out
}

// assertKnownVariants records a proven variant as a fresh path fact at a join
// point: the fact held at the end of every incoming edge and no code runs
// between those edges and the join, so it holds here. It is version-tracked
// from here on like any other path fact and ends with the enclosing block.
func (a *Analyzer) assertKnownVariants(set knownVariantSet) {
	bindings := make([]string, 0, len(set))
	for binding := range set {
		bindings = append(bindings, binding)
	}
	sort.Strings(bindings)
	for _, binding := range bindings {
		variant := set[binding]
		symbol, exists := a.symbols.lookup(binding)
		if !exists {
			continue
		}
		if value, known := a.knownVariantTest(binding, variant); known && value {
			continue
		}
		token := symbol.Token
		condition := &ast.StateTestExpression{
			Token:   token,
			Subject: &ast.Identifier{Token: token, Value: binding},
			Variant: &ast.Identifier{Token: token, Value: variant},
		}
		a.activeConditionFacts = append(a.activeConditionFacts, a.activeProofFact(ResolvedConditionFact{Kind: ConditionFactBranchTrue, Condition: condition}))
	}
}

// constructedVariantOf returns the variant a value expression directly
// constructs for a binding of the given type: Ok/Err, Some(...)/None, or a
// declared union's variant constructor.
func (a *Analyzer) constructedVariantOf(value ast.Expression, typ Type) string {
	switch value := value.(type) {
	case *ast.OkExpression:
		return "Ok"
	case *ast.ErrExpression:
		return "Err"
	case *ast.Identifier:
		if isOptionType(typ) && value != nil && value.Value == "None" {
			return "None"
		}
	case *ast.CallExpression:
		if callee, ok := value.Callee.(*ast.Identifier); ok && isOptionType(typ) && callee != nil && callee.Value == "Some" {
			return "Some"
		}
	}
	return a.unionConstructorVariant(value, typ)
}

// recordAssignedVariant proves the variant a direct assignment just stored
// in a binding (`x = Some(1)`, `c = Choice.No`).
func (a *Analyzer) recordAssignedVariant(stmt *ast.AssignmentStatement) {
	target, ok := stmt.Target.(*ast.Identifier)
	if !ok || target == nil || stmt.Operator != "=" {
		return
	}
	symbol, exists := a.symbols.lookup(target.Value)
	if !exists || symbol.Volatile || symbol.Type.Kind == ReferenceType {
		return
	}
	if variant := a.constructedVariantOf(stmt.Value, symbol.Type); variant != "" {
		a.assertKnownVariants(knownVariantSet{target.Value: variant})
	}
}

// recordBlockExitKnownVariants keeps the proven variants at the end of a
// block for the join that follows it.
func (a *Analyzer) recordBlockExitKnownVariants(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	if a.blockExitKnownVariants == nil {
		a.blockExitKnownVariants = map[*ast.BlockStatement]knownVariantSet{}
	}
	a.blockExitKnownVariants[block] = a.currentKnownVariants()
}

// knownVariantsAfterNegation is the proven set on the path where condition
// is false, without leaving the negated fact behind.
func (a *Analyzer) knownVariantsAfterNegation(condition ast.Expression) knownVariantSet {
	mark := len(a.activeConditionFacts)
	a.recordPathConditionFact(condition, false)
	out := a.currentKnownVariants()
	a.activeConditionFacts = a.activeConditionFacts[:mark]
	return out
}
