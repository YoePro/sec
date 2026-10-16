package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// mutexAcquisition connects local guard ownership to a canonical resource
// observation. Invalidated proofs stay invalid across speculative branches:
// uncertain replacement must weaken identity, never manufacture self-lock.
// Rules: rules/concurrency/mutex.md — §§28,42–43;
// rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity" and "Ownership and resource transfer".
type mutexAcquisition struct {
	call        *ast.CallExpression
	resource    SynchronizationResourceFact
	owner       CallableID
	invalidated bool
}

// mutexAcquisitionFromExpression retains acquisition provenance through local
// moves, aggregate construction and successful try extraction. The first
// retained acquisition also supplies an escape diagnostic's ownership witness.
// Optional/fallible acquisitions become
// held-resource proofs only when their guard payload is actually bound.
// Rules: rules/concurrency/mutex.md — §§20–24,28,39–40,42–43.
func (a *Analyzer) mutexAcquisitionFromExpression(expr ast.Expression) *mutexAcquisition {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return a.symbols[expr.Value].mutexAcquisition
	case *ast.RefExpression:
		return a.mutexAcquisitionFromExpression(expr.Value)
	case *ast.StructLiteral:
		for _, field := range expr.Fields {
			if field != nil {
				if acquisition := a.mutexAcquisitionFromExpression(field.Value); acquisition != nil {
					return acquisition
				}
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if acquisition := a.mutexAcquisitionFromExpression(element); acquisition != nil {
				return acquisition
			}
		}
	case *ast.OkExpression:
		return a.mutexAcquisitionFromExpression(expr.Value)
	case *ast.ErrExpression:
		return a.mutexAcquisitionFromExpression(expr.Value)
	case *ast.PrefixExpression:
		if expr.Operator == "<-" {
			return a.mutexAcquisitionFromExpression(expr.Right)
		}
	case *ast.TryExpression:
		// Recovery may substitute a guard belonging to another resource.
		if len(expr.Handlers) == 0 {
			return a.mutexAcquisitionFromExpression(expr.Expression)
		}
		plan, ok := a.ResolvedTryPlanOf(expr)
		if !ok {
			return nil
		}
		for _, handler := range plan.Handlers {
			if handler.Flow != TryHandlerReturns && handler.Flow != TryHandlerTerminates {
				return nil
			}
		}
		return a.mutexAcquisitionFromExpression(expr.Expression)
	case *ast.CallExpression:
		if _, payload := a.unionConstructorPayload(expr); payload {
			return a.mutexAcquisitionFromExpression(expr.Arguments[0])
		}
		member, ok := expr.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil || (member.Property.Value != "Lock" && member.Property.Value != "TryLock") {
			return nil
		}
		if typ := dereferenceType(a.expressionTypes[member.Object]); !isMutexType(typ) {
			return nil
		}
		if a.expressionTypes[expr].Kind == InvalidType {
			return nil
		}
		if resource, ok := a.SynchronizationResourceOf(member.Object); ok {
			return &mutexAcquisition{call: expr, resource: resource, owner: a.currentCallable}
		}
	}
	return nil
}

// bindMutexAcquisition follows a validated binding transfer before its source
// is marked moved. Keeping the fact on the lexical symbol gives ordinary
// scope exit, branch isolation and moved-state checks their usual authority.
// Rules: rules/concurrency/mutex.md — §§28(3),37,42–43.
func (a *Analyzer) bindMutexAcquisition(name string, value ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || a.summaryPass {
		return
	}
	symbol.mutexAcquisition = a.mutexAcquisitionFromExpression(value)
	a.symbols[name] = symbol
}

// bindMutexPayloadAcquisition makes a successful acquisition's guard payload
// available to the same proof in match and if-let success branches.
// Rules: rules/concurrency/mutex.md — §§22–24,42–43;
// rules/control-flow/flowcontrol_match.md — §§12–14.
func (a *Analyzer) bindMutexPayloadAcquisition(name, subject string) {
	symbol := a.symbols[name]
	if isMutexGuardType(dereferenceType(symbol.Type)) {
		symbol.mutexAcquisition = a.symbols[subject].mutexAcquisition
		a.symbols[name] = symbol
	}
}

// checkMutexReacquisition rejects only a proven same-resource acquisition
// while a local owner still holds its guard in this callable. Unknown aliases,
// other execution entities and unextracted Option/Result values are not proofs.
// Both blocking Lock and nonblocking TryLock violate non-reentrancy.
// Rules: rules/concurrency/mutex.md — §§42(1–3),43(1–4),63;
// rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func (a *Analyzer) checkMutexReacquisition(call *ast.CallExpression, member *ast.MemberExpression) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	if _, ok := a.resolvePlace(member.Object); !ok {
		return
	}
	resource, ok := a.SynchronizationResourceOf(member.Object)
	if !ok {
		return
	}
	current := a.resourceIdentityDomain.Identify(resource)
	var previous *mutexAcquisition
	for name, symbol := range a.symbols {
		held := symbol.mutexAcquisition
		if !isMutexGuardType(dereferenceType(symbol.Type)) || held == nil || held.invalidated || held.call == call || held.owner != a.currentCallable || !a.assigned[name] {
			continue
		}
		place, ok := a.rootPlace(name)
		if !ok {
			continue
		}
		if _, _, _, unavailable := a.unavailablePlace(place); unavailable {
			continue
		}
		if SynchronizationRelationship(current, a.resourceIdentityDomain.Identify(held.resource)) != ResourceSame {
			continue
		}
		if previous == nil || mutexAcquisitionBefore(held.call.Token, previous.call.Token) {
			previous = held
		}
	}
	if previous != nil {
		before := len(a.errors)
		a.addErrorAtTokenWithPrevious(member.Property.Token, previous.call.Token,
			"mutex %s is already locked by the current execution entity", member.Object.String())
		if len(a.errors) > before {
			a.errors[len(a.errors)-1].RelatedLabel = "previous acquisition; guard is still live"
		}
	}
}

// mutexAcquisitionBefore chooses a stable diagnostic witness independent of
// symbol-map iteration order (rules/tooling/diagnostics.md — §8).
func mutexAcquisitionBefore(left, right lexer.Token) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.Column < right.Column
}

// invalidateMutexAcquisitionsForWrite drops provenance when replacement or a
// callee's mutable borrow may change the observed resource. A
// branch write also invalidates its shared pre-branch proof conservatively.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity", "Ownership and resource transfer"; rules/concurrency/mutex.md — §46.
func (a *Analyzer) invalidateMutexAcquisitionsForWrite(expr ast.Expression) {
	if a.summaryPass {
		return
	}
	if ref, ok := expr.(*ast.RefExpression); ok {
		expr = ref.Value
	}
	place, ok := a.resolvePlace(expr)
	if !ok {
		return
	}
	if place.Type.Kind == ReferenceType {
		place = a.canonicalDereferencePlace(expr, place, place.Type)
	}
	for _, symbol := range a.symbols {
		if held := symbol.mutexAcquisition; held != nil {
			holder, ok := a.rootPlace(symbol.Name)
			if placesMayOverlap(place, held.resource.place) || (ok && placesMayOverlap(place, holder)) {
				held.invalidated = true
			}
		}
	}
}
