package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// creationOwnerType identifies a Result whose success payload owns a child.
// Rules: rules/concurrency/tasks.md §4; rules/concurrency/threads.md §2.
func creationOwnerType(typ Type) (Type, bool) {
	if typ.Kind == ResultType && len(typ.TypeArgs) == 2 {
		owner := typ.TypeArgs[0]
		return owner, isTaskType(owner) || isThreadType(owner)
	}
	return Type{}, false
}

// creationLifecycleResolved uses canonical variant and Place facts. A conditional
// move is insufficient: another continuing path may still own a successful child.
// Rules: rules/memory/ownership.md §§5–6,20–21;
// rules/concurrency/tasks.md §4(4–6); rules/concurrency/threads.md §§2,34;
// rules/control-flow/discard.md §§16,24.
func (a *Analyzer) creationLifecycleResolved(name string) bool {
	if assigned, tracked := a.assigned[name]; tracked && !assigned {
		return true
	}
	if _, resolved := a.closedResources[name]; resolved {
		return true
	}
	if failure, known := a.knownVariantTest(name, "Err"); known && failure {
		return true
	}
	if a.provesResultProjectionSafe(&ast.Identifier{Value: name}, "Err") {
		return true
	}
	payload := Place{Root: name, Projections: []PlaceProjection{{Kind: PlaceUnionPayload, Name: "Ok"}}}
	_, key, partial, unavailable := a.unavailablePlace(payload)
	return unavailable && !partial && !isConditionallyUnavailable(a.availabilityStates[key])
}

// recordResolvedCreationLifecycles retains a discharged child obligation before
// path refinements leave scope. Existing lifecycle snapshots intersect these
// facts across continuing paths without marking an Err carrier as moved.
// Reinitialization or new mutable authority invalidates this fact.
// Rules: rules/memory/ownership.md §§20–21; rules/concurrency/tasks.md §§4–5;
// rules/concurrency/threads.md §§2,34.
func (a *Analyzer) recordResolvedCreationLifecycles() {
	for name, symbol := range a.symbols.all() {
		if _, creation := creationOwnerType(symbol.Type); symbol.Local && creation && a.creationLifecycleResolved(name) {
			a.closedResources[name] = symbol.Token
		}
	}
}

// checkCreationLifecyclesAtReturn checks every visible carrier on an early exit,
// after the return expression has transferred any returned owner.
// Rules: rules/memory/ownership.md §17; rules/concurrency/tasks.md §30;
// rules/concurrency/threads.md §34.
func (a *Analyzer) checkCreationLifecyclesAtReturn() {
	names := []string{}
	for name, symbol := range a.symbols.all() {
		if _, creation := creationOwnerType(symbol.Type); symbol.Local && creation {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		a.checkUnresolvedTaskAtScopeExit(name)
	}
}

// pendingCreationLifecycles snapshots outstanding carriers before try handlers
// change success-path state. An unhandled failure exits with these obligations.
// Rules: rules/errors/errorhandling.md §§12.1,16; rules/memory/ownership.md §§17,20–21.
func (a *Analyzer) pendingCreationLifecycles() []Symbol {
	var pending []Symbol
	for name, symbol := range a.symbols.all() {
		if _, creation := creationOwnerType(symbol.Type); symbol.Local && creation && !a.creationLifecycleResolved(name) {
			pending = append(pending, symbol)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Name < pending[j].Name })
	return pending
}

// reportUnresolvedCreation diagnoses a carrier without consuming it or inventing
// a success state; repeated exits share one primary diagnostic at its binding.
// Rules: rules/concurrency/tasks.md §30; rules/concurrency/threads.md §34;
// rules/tooling/diagnostics.md §§8–9,21.
func (a *Analyzer) reportUnresolvedCreation(symbol Symbol) {
	owner, _ := creationOwnerType(symbol.Type)
	kind := "task"
	if isThreadType(owner) {
		kind = "thread"
	}
	message := fmt.Sprintf("creation result %s may contain an owned %s unresolved at scope exit", symbol.Name, kind)
	for _, previous := range a.errors {
		if previous.ID == diagnostics.UnresolvedCreationLifecycle && previous.File == symbol.Token.File && previous.Line == symbol.Token.Line && previous.Column == symbol.Token.Column && previous.Message == message {
			return
		}
	}
	a.addErrorAtTokenWithMetadata(symbol.Token, diagnostics.UnresolvedCreationLifecycle,
		"Handle the Result with try or match, then join, await or legally transfer the successful owner; alternatively return or transfer the complete Result. Creation failure produces no child.", "%s", message)
}

// checkUnresolvedTaskAtScopeExit prevents a child owner or its creation Result
// from disappearing at a scope exit before join, await or legal transfer.
// Rules: rules/concurrency/tasks.md — §§4–5,30;
// rules/concurrency/threads.md — §§2,34; rules/control-flow/discard.md — §9.
func (a *Analyzer) checkUnresolvedTaskAtScopeExit(name string) {
	symbol, ok := a.symbols.lookup(name)
	if !ok {
		return
	}
	if _, creation := creationOwnerType(symbol.Type); creation {
		if a.creationLifecycleResolved(name) {
			return
		}
		a.reportUnresolvedCreation(symbol)
		return
	}
	if !isTaskType(symbol.Type) && !isThreadType(symbol.Type) {
		return
	}
	if _, moved := a.moved[name]; moved {
		return
	}
	if _, joined := a.closedResources[name]; joined {
		// threads.md § 34, tasks.md § 17: a joined handle's lifecycle is resolved.
		return
	}
	kind := "task"
	if isThreadType(symbol.Type) {
		kind = "thread"
	}
	a.addErrorAtToken(symbol.Token, "owned %s %s is unresolved at scope exit", kind, name)
}
