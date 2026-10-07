package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ResolvedStateCheck is a canonical, currently valid path proof for one binding.
// Source names the state test or pattern, rather than the declaration spelling.
// Rules: rules/errors/errorhandling.md — §§6.1–6.2; rules/declarations/unions.md — §8.
type ResolvedStateCheck struct {
	Binding ResolvedBinding
	Variant string
	Source  lexer.Token
}

// ResolvedStateRequirement records a state-sensitive operation's owning proof
// obligation. Checks describe only facts established on this operation's path;
// a state proof about another binding does not satisfy the requirement.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls";
// rules/errors/errorhandling.md — §6.1 "Consuming projections".
type ResolvedStateRequirement struct {
	Binding ResolvedBinding
	Variant string
	Proven  bool
	Checks  []ResolvedStateCheck
}

// cloneStateRequirement detaches type and check data before publication.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results.
func cloneStateRequirement(fact ResolvedStateRequirement) ResolvedStateRequirement {
	fact.Binding.Type = semanticSnapshotType(fact.Binding.Type)
	fact.Checks = append([]ResolvedStateCheck(nil), fact.Checks...)
	for i := range fact.Checks {
		fact.Checks[i].Binding.Type = semanticSnapshotType(fact.Checks[i].Binding.Type)
	}
	return fact
}

// ResolvedStateRequirementOf returns the owning operation's immutable state
// facts, including independent construction/path proof and localized uncertainty.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls".
func (a *Analyzer) ResolvedStateRequirementOf(expr ast.Expression) (ResolvedStateRequirement, bool) {
	if a == nil {
		return ResolvedStateRequirement{}, false
	}
	fact, found := a.resolvedStateRequirements[expr]
	return cloneStateRequirement(fact), found
}

// recordResultStateRequirement publishes the obligation only for an owned,
// canonically resolved binding. Repeated loop visits keep the weakest proof and
// the intersection of supporting checks, never facts from incompatible visits.
// Rules: rules/errors/errorhandling.md — §6.1; rules/analysis/pitfall_analysis.md —
// "Option, Result, and state-correlation pitfalls", "Canonical facts consumed by pitfall analysis".
func (a *Analyzer) recordResultStateRequirement(operation ast.Expression, receiver ast.Expression, variant string, proven bool) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	identifier, ok := receiver.(*ast.Identifier)
	if !ok {
		return
	}
	binding, ok := a.ResolvedBindingOf(identifier)
	if !ok || binding.ID == 0 || binding.Type.Kind != ResultType {
		return
	}
	fact := ResolvedStateRequirement{Binding: binding, Variant: variant, Proven: proven, Checks: a.activeResultStateChecks()}
	if previous, found := a.resolvedStateRequirements[operation]; found {
		fact.Proven = previous.Proven && fact.Proven
		kept := fact.Checks[:0]
		for _, check := range fact.Checks {
			for _, old := range previous.Checks {
				if check.Binding.ID == old.Binding.ID && check.Variant == old.Variant && sourceTokenLocation(check.Source) == sourceTokenLocation(old.Source) {
					kept = append(kept, check)
					break
				}
			}
		}
		fact.Checks = kept
	}
	a.resolvedStateRequirements[operation] = cloneStateRequirement(fact)
}

// activeResultStateChecks consumes the canonical path/epoch state and owning
// Result proof service. Mutable, expired, moved and unresolved subjects cannot
// supply an advisory proof, and construction alone is not a checked condition.
// Rules: rules/errors/errorhandling.md — §§6.1–6.2; rules/analysis/pitfall_analysis.md —
// "Option, Result, and state-correlation pitfalls", "Guards participate in pitfall reasoning".
func (a *Analyzer) activeResultStateChecks() []ResolvedStateCheck {
	var checks []ResolvedStateCheck
	add := func(name, variant string, source lexer.Token) {
		symbol, exists := a.symbols[name]
		if !exists || symbol.Mutable || symbol.Type.Kind != ResultType || !validDefinitionToken(source) {
			return
		}
		if _, moved := a.moved[name]; moved {
			return
		}
		identifier := &ast.Identifier{Token: source, Value: name}
		if !a.provesResultProjectionSafe(identifier, variant) {
			return
		}
		binding := a.bindingFacts[sourceTokenLocation(symbol.Token)]
		if binding.ID == 0 {
			return
		}
		for _, old := range checks {
			if old.Binding.ID == binding.ID && old.Variant == variant {
				return
			}
		}
		checks = append(checks, ResolvedStateCheck{Binding: binding, Variant: variant, Source: source})
	}
	for _, active := range a.activeConditionFacts {
		if active.epoch != a.arrayIndexMutationEpoch || active.fact.Kind == ConditionFactLogicalRHSFalse {
			continue
		}
		if active.resultBinding != "" {
			add(active.resultBinding, active.resultState, active.resultSource)
			continue
		}
		a.collectConditionResultStateChecks(active.fact.Condition, add)
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Source.File != checks[j].Source.File {
			return checks[i].Source.File < checks[j].Source.File
		}
		if checks[i].Source.Line != checks[j].Source.Line {
			return checks[i].Source.Line < checks[j].Source.Line
		}
		if checks[i].Source.Column != checks[j].Source.Column {
			return checks[i].Source.Column < checks[j].Source.Column
		}
		return checks[i].Binding.ID < checks[j].Binding.ID
	})
	return checks
}

// collectConditionResultStateChecks normalizes admitted Result variant tests
// and compiler-known borrowed-projection Option checks from canonical true path
// conditions. Disjunctions establish no single binding state and are excluded.
// Rules: rules/declarations/unions.md — §8; rules/errors/errorhandling.md — §6.2;
// rules/analysis/pitfall_analysis.md — "Semantic recognition, not syntax matching".
func (a *Analyzer) collectConditionResultStateChecks(condition ast.Expression, add func(string, string, lexer.Token)) {
	switch condition := condition.(type) {
	case *ast.InfixExpression:
		if condition.Operator == "&&" {
			a.collectConditionResultStateChecks(condition.Left, add)
			a.collectConditionResultStateChecks(condition.Right, add)
		}
	case *ast.StateTestExpression:
		identifier, ok := condition.Subject.(*ast.Identifier)
		if !ok || condition.Empty || condition.Variant == nil {
			return
		}
		fact, validated := a.resolvedStateTests[condition]
		if !validated {
			// False-path normalization preserves the original subject and source
			// while creating a negated syntax node. Consume its validated owner.
			for original, candidate := range a.resolvedStateTests {
				if original.Subject == condition.Subject && sourceTokenLocation(original.Token) == sourceTokenLocation(condition.Token) {
					fact, validated = candidate, true
					break
				}
			}
		}
		if !validated || fact.SubjectType.Kind != ResultType || fact.Empty {
			return
		}
		variant := condition.Variant.Value
		if variant != "Ok" && variant != "Err" {
			return
		}
		if condition.Negated {
			variant = oppositeResultState(variant)
		}
		add(identifier.Value, variant, condition.Token)
	case *ast.MatchExpression:
		typ, typed := a.ResolvedTypeOf(condition.Subject)
		if !typed || typ.Kind != UnionType || typ.Name != "Option" {
			return
		}
		name, present, ok := borrowedProjectionState(condition.Subject)
		if !ok {
			return
		}
		_, none, ok := optionStateTest(condition, name)
		if !ok {
			return
		}
		if none {
			present = oppositeResultState(present)
		}
		add(name, present, condition.Token)
	}
}
