package sema

import (
	"sort"

	"sec/internal/lexer"
)

// ResolvedDestructionFact is the tooling view of one owned binding's
// destruction responsibility (destruction.md §§ 3, 6.1, 9–12, 33): the cleanup
// registration with its destruction plan graph, how the obligation
// originated, the validated terminal uses that end all or part of it, and
// the defer bodies that still use the binding before it is destroyed. It is
// assembled from published facts only; it does not resolve exit paths.
type ResolvedDestructionFact struct {
	Registration ResolvedOwnedCleanupRegistration
	// Origin is "declaration", "parameter" or "pattern binding".
	Origin string
	// Terminals are the terminal uses of the binding in source order.
	Terminals []ResolvedOwnedCleanupCancellation
	// DeferredUses are defer-body uses of the binding in source order.
	DeferredUses []lexer.Token
}

// noteDeferredUse retains a defer-body use of an outer binding.
func (a *Analyzer) noteDeferredUse(place Place, token lexer.Token) {
	if a.summaryPass || place.RootToken.Line == 0 || token.Line == 0 {
		return
	}
	if a.deferredUses == nil {
		a.deferredUses = map[sourceTokenKey][]lexer.Token{}
	}
	key := sourceTokenLocation(place.RootToken)
	a.deferredUses[key] = append(a.deferredUses[key], token)
}

// ResolvedDestructionAt returns the destruction fact of the owned binding
// declared at a source position.
// Rules: rules/memory/destruction.md — §§ 3, 6.1, 9–12, 33; rules/tooling/lsp.md — "Hover".
func (a *Analyzer) ResolvedDestructionAt(file string, line, column int) (ResolvedDestructionFact, bool) {
	if a == nil {
		return ResolvedDestructionFact{}, false
	}
	here := sourceTokenKey{File: file, Line: line, Column: column}
	fact, found := ResolvedDestructionFact{}, false
	consider := func(registration ResolvedOwnedCleanupRegistration, origin string) {
		if !found && sourceTokenLocation(registration.Place.RootToken) == here {
			fact, found = ResolvedDestructionFact{Registration: cloneOwnedCleanupRegistration(registration), Origin: origin}, true
		}
	}
	for _, registration := range a.ownedCleanupInitializations {
		consider(registration, "declaration")
	}
	for _, parameter := range a.ownedCleanupParameters {
		consider(parameter.registration, "parameter")
	}
	for _, registration := range a.ownedCleanupMatchBindings {
		consider(registration, "pattern binding")
	}
	for _, registration := range a.ownedCleanupHandlerBindings {
		consider(registration, "pattern binding")
	}
	if !found {
		return ResolvedDestructionFact{}, false
	}
	for _, cancellations := range a.ownedCleanupCancellations {
		for _, cancellation := range cancellations {
			if cancellation.Binding == fact.Registration.Binding {
				cancellation.Place = cloneSlicePlanPlace(cancellation.Place)
				fact.Terminals = append(fact.Terminals, cancellation)
			}
		}
	}
	sort.Slice(fact.Terminals, func(i, j int) bool { return tokenBefore(fact.Terminals[i].Source, fact.Terminals[j].Source) })
	fact.DeferredUses = append([]lexer.Token(nil), a.deferredUses[here]...)
	sort.Slice(fact.DeferredUses, func(i, j int) bool { return tokenBefore(fact.DeferredUses[i], fact.DeferredUses[j]) })
	return fact, true
}
