package sema

import (
	"reflect"
	"sec/internal/ast"
	"sec/internal/lexer"
)

// activeProofFact captures versions of the canonical condition's local reads.
// Local shadowing is forbidden, so binding names identify these dependencies.
// Versions are monotonic across branch analysis: a write on either branch
// prevents an old fact from surviving the join. Loop entry is a full barrier.
// Rules: rules/errors/panic.md — §§15.6(1),(3),15.8(1);
// rules/foundations/names_scopes_visibility.md — local shadowing;
// rules/control-flow/flowcontrol_if.md — branch joins.
func (a *Analyzer) activeProofFact(fact ResolvedConditionFact) activeConditionFact {
	active := activeConditionFact{fact: fact, epoch: a.arrayIndexMutationEpoch,
		proofTracked: true, proofBarrier: a.conditionProofBarrier, bindingVersions: map[string]uint64{}}
	walkASTValue(reflect.ValueOf(fact.Condition), func(node any) {
		if id, ok := node.(*ast.Identifier); ok {
			if _, exists := a.symbols[id.Value]; exists {
				active.bindingVersions[id.Value] = a.conditionBindingVersions[id.Value]
			}
		}
	})
	return active
}

// assertionFactValid checks only the bindings read by an assertion/refinement.
// Legacy non-condition state facts still obey their conservative epoch.
// Rules: rules/errors/panic.md — §§15.6,15.8(1).
func (a *Analyzer) assertionFactValid(active activeConditionFact) bool {
	if !active.proofTracked {
		return active.epoch == a.arrayIndexMutationEpoch
	}
	if active.proofBarrier != a.conditionProofBarrier {
		return false
	}
	for name, version := range active.bindingVersions {
		if a.conditionBindingVersions[name] != version {
			return false
		}
		if symbol, ok := a.symbols[name]; !ok || symbol.Volatile {
			return false
		}
	}
	return true
}

// invalidateAssignmentProofFacts invalidates the written scalar binding.
// Reference/member/index writes and effectful RHS calls remain full barriers:
// a setter or alias can affect other reads, so no independence is guessed.
// Rules: rules/errors/panic.md — §15.8(1) proof on every path;
// rules/memory/borrowing.md — mutable references and exclusive access.
func (a *Analyzer) invalidateAssignmentProofFacts(stmt *ast.AssignmentStatement) {
	id, direct := stmt.Target.(*ast.Identifier)
	symbol, known := Symbol{}, false
	if direct {
		symbol, known = a.symbols[id.Value]
	}
	uncertain := !direct || !known || symbol.Type.Kind == ReferenceType
	if direct {
		_, property := a.lookupCurrentImplProperty(id.Value)
		uncertain = uncertain || property
	}
	walkASTValue(reflect.ValueOf(stmt.Value), func(node any) {
		if call, ok := node.(*ast.CallExpression); ok && !a.isExplicitConversionExpression(call) {
			uncertain = true
		}
	})
	if uncertain {
		a.conditionProofBarrier++
		return
	}
	if a.conditionBindingVersions == nil {
		a.conditionBindingVersions = map[string]uint64{}
	}
	a.conditionBindingVersions[id.Value]++
}

// invalidateCallProofFacts prevents a comparison surviving a call that may
// mutate a read through aliases or globals. Scalar conversions have no such
// writes; a trusted noPanic contract alone does not establish mutation freedom.
// Rules: rules/errors/panic.md — §15.8(1); rules/memory/borrowing.md — mutation authority.
func (a *Analyzer) invalidateCallProofFacts(call *ast.CallExpression) {
	if call != nil && !a.isExplicitConversionExpression(call) {
		a.conditionProofBarrier++
	}
}

// activeConditionFact adds mutation-generation validity to the public
// condition fact while it dominates the current analysis point.
type activeConditionFact struct {
	fact            ResolvedConditionFact
	epoch           uint64
	proofTracked    bool
	proofBarrier    uint64
	bindingVersions map[string]uint64
	// resultBinding and resultState carry a Result state fact ("Ok" or
	// "Err") of an immutable binding that a path has proven without a
	// condition of its own: an else branch, an early exit, an Option
	// binding, or a match arm on a borrowed projection.
	resultBinding string
	resultState   string
	resultSource  lexer.Token
}

// recordConditionFact records and activates the common truth fact used by
// successful assertions and if true branches. Mutation generation is kept
// internal while the source-level provenance remains queryable and immutable.
//
// Rules:
//   - rules/errors/panic.md — § 15.6 "Assertion refinement"
//   - rules/control-flow/flowcontrol_if.md — § 27 "Sema and flow-analysis requirements"
func (a *Analyzer) recordConditionFact(condition ast.Expression, kind ResolvedConditionFactKind, source lexer.Token) ResolvedConditionFact {
	fact := ResolvedConditionFact{Kind: kind, Condition: condition, Source: source}
	a.resolvedConditionFacts[condition] = fact
	a.activeConditionFacts = append(a.activeConditionFacts, a.activeProofFact(fact))
	return fact
}
