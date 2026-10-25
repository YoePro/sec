package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// CleanupActionID, CleanupScopeID and DeferID identify compiler semantic sites,
// never source values or runtime registrations. Zero denotes an absent identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§35–38,42–44.
type CleanupActionID uint32
type CleanupScopeID uint32
type DeferID uint32

type cleanupScopeKey struct {
	Parent CleanupScopeID
	Site   sourceTokenKey
}

type cleanupActionKey struct {
	Binding BindingID
	Site    sourceTokenKey
	Defer   DeferID
}

// ResolvedDeferIdentity identifies a defer site and its lexical scope. Each
// execution still registers a distinct dynamic instance at function scope;
// these IDs do not encode registration, execution order or an active flag.
// Rules: rules/control-flow/defer.md — §§12–14;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,42–44.
type ResolvedDeferIdentity struct {
	Defer  DeferID
	Action CleanupActionID
	Scope  CleanupScopeID
}

// cleanupIdentityAllocator interns sites in deterministic analysis order.
// An owned action is keyed by canonical BindingID and initialization site:
// reinitialization must never reuse the previous incarnation's action. Actual
// registration/cancellation and dynamic multiplicity belong to P17-19..24.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,38,43.
type cleanupIdentityAllocator struct {
	scopes  map[cleanupScopeKey]CleanupScopeID
	actions map[cleanupActionKey]CleanupActionID
	defers  map[cleanupScopeKey]DeferID
}

// cleanupSiteIdentity interns an already validated semantic key from one.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §36.
func cleanupSiteIdentity[K comparable, ID ~uint32](table *map[K]ID, key K) ID {
	if *table == nil {
		*table = map[K]ID{}
	}
	if id, exists := (*table)[key]; exists {
		return id
	}
	id := ID(len(*table) + 1)
	(*table)[key] = id
	return id
}

// scope identifies a lexical block, retaining its enclosing scope identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,39.
func (a *cleanupIdentityAllocator) scope(parent CleanupScopeID, site lexer.Token) CleanupScopeID {
	if !validDefinitionToken(site) {
		return 0
	}
	return cleanupSiteIdentity(&a.scopes, cleanupScopeKey{parent, sourceTokenLocation(site)})
}

// owned identifies one binding incarnation's prospective destruction action.
// It does not register an obligation or claim initialization has succeeded.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,38;
// rules/memory/destruction.md — §6.1 "Automatic destruction registration".
func (a *cleanupIdentityAllocator) owned(binding BindingID, initialization lexer.Token) CleanupActionID {
	if binding == 0 || !validDefinitionToken(initialization) {
		return 0
	}
	return cleanupSiteIdentity(&a.actions, cleanupActionKey{Binding: binding, Site: sourceTokenLocation(initialization)})
}

// deferred identifies a valid defer site without collapsing its executions.
// Rules: rules/control-flow/defer.md — §§12–14;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,42–44.
func (a *cleanupIdentityAllocator) deferred(scope CleanupScopeID, site lexer.Token) ResolvedDeferIdentity {
	if scope == 0 || !validDefinitionToken(site) {
		return ResolvedDeferIdentity{}
	}
	id := cleanupSiteIdentity(&a.defers, cleanupScopeKey{scope, sourceTokenLocation(site)})
	action := cleanupSiteIdentity(&a.actions, cleanupActionKey{Defer: id})
	return ResolvedDeferIdentity{Defer: id, Action: action, Scope: scope}
}

// enterCleanupScope records a lexical identity while analyzing a block; its
// returned closure restores the enclosing scope even on early analysis exit.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,39.
func (a *Analyzer) enterCleanupScope(block *ast.BlockStatement) func() {
	previous := a.currentCleanupScope
	id := a.cleanupIdentities.scope(previous, block.Token)
	if a.cleanupScopes == nil {
		a.cleanupScopes = map[*ast.BlockStatement]CleanupScopeID{}
	}
	a.cleanupScopes[block] = id
	a.currentCleanupScope = id
	a.recordCleanupScope(id, previous, block)
	return func() {
		a.recordCleanupBlockExit(id, block)
		a.currentCleanupScope = previous
	}
}

// ResolvedCleanupScopeOf reads a block's analyzed lexical identity without
// allocating or inferring anything. Unknown blocks have no identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,79.
func (a *Analyzer) ResolvedCleanupScopeOf(block *ast.BlockStatement) (CleanupScopeID, bool) {
	if a == nil {
		return 0, false
	}
	id := a.cleanupScopes[block]
	return id, id != 0
}

// ResolvedDeferIdentityOf reads a validated defer's site identities. This is
// not a cleanup plan or a claim that the statement executes on every path.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,42–44,79.
func (a *Analyzer) ResolvedDeferIdentityOf(stmt *ast.DeferStatement) (ResolvedDeferIdentity, bool) {
	if a == nil {
		return ResolvedDeferIdentity{}, false
	}
	id, ok := a.cleanupDefers[stmt]
	return id, ok
}
