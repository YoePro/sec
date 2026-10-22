package sema

import "sec/internal/lexer"

// ResolvedOwnedCleanupCancellation ends the current ownership responsibility
// at a validated terminal use. A projected Place cancels only that subobject;
// it never cancels the remaining owner's whole cleanup. Binding/Place select
// the current incarnation on the executing path, not one guessed registration
// from another predecessor. Resolving those incarnations belongs to cleanup CFG
// planning. This fact itself neither destroys nor transfers the value.
// Rules: rules/memory/destruction.md — §6.1(3), §§9–12;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§38,59,73,80.
type ResolvedOwnedCleanupCancellation struct {
	Binding    BindingID
	Place      Place
	Invocation CallableID
	Source     lexer.Token
	Reason     string
}

// recordOwnedCleanupCancellation retains an already accepted ownership end.
// Repeated analysis of the same site is idempotent; identity comparisons use
// the canonical Place rather than its diagnostic spelling. Borrow-holder end
// does not create a referent-destruction cancellation.
// Rules: rules/memory/destruction.md — §6.1(3), §18;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§27,38,73.
func (a *Analyzer) recordOwnedCleanupCancellation(place Place, source lexer.Token, reason string) {
	if TriviallyDestructible(place.Type) || !validDefinitionToken(source) {
		return
	}
	binding := a.bindingIDs[sourceTokenLocation(place.RootToken)]
	if binding == 0 {
		return
	}
	key := sourceTokenLocation(source)
	for _, existing := range a.ownedCleanupCancellations[key] {
		// Resource and general ownership trackers may observe the same end.
		// Retain its first provenance rather than publishing two cancellations.
		if existing.Binding == binding && existing.Invocation == a.currentCallable && samePlaceIdentity(existing.Place, place) {
			return
		}
	}
	if a.ownedCleanupCancellations == nil {
		a.ownedCleanupCancellations = map[sourceTokenKey][]ResolvedOwnedCleanupCancellation{}
	}
	a.ownedCleanupCancellations[key] = append(a.ownedCleanupCancellations[key], ResolvedOwnedCleanupCancellation{
		Binding: binding, Place: cloneSlicePlanPlace(place), Invocation: a.currentCallable, Source: source, Reason: reason,
	})
}

// ResolvedOwnedCleanupCancellationsAt returns detached terminal-use facts in
// semantic producer order for one source location, without changing flow state.
// Multiple subobjects at a single site remain separate cancellations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§73,79–80.
func (a *Analyzer) ResolvedOwnedCleanupCancellationsAt(file string, line, column int) []ResolvedOwnedCleanupCancellation {
	if a == nil {
		return nil
	}
	stored := a.ownedCleanupCancellations[sourceTokenKey{File: file, Line: line, Column: column}]
	if stored == nil {
		return nil
	}
	plans := make([]ResolvedOwnedCleanupCancellation, len(stored))
	copy(plans, stored)
	for index := range plans {
		plans[index].Place = cloneSlicePlanPlace(plans[index].Place)
	}
	return plans
}
