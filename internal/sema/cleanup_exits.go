package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// Cleanup exit edges (P17-21 … P17-23; package17 §§ 39–40, 44, 46–47;
// destruction.md §§ 6–8). Analysis records the lexical scope tree, the loop
// scopes break/continue leave, and every language-controlled exit with the
// scopes it leaves, innermost first. A function exit (return, error
// propagation, falling off the body) also runs the remaining function-scoped
// registrations, including defers; a panic or unreachable runs no cleanup
// (no unwinding, § 46). The returned or propagated value is protected: its
// ownership leaves before cleanup runs (§ 40).

// CleanupExitKind is the language-controlled way control leaves scopes.
type CleanupExitKind string

const (
	CleanupExitFallthrough      CleanupExitKind = "fallthrough"
	CleanupExitBreak            CleanupExitKind = "break"
	CleanupExitContinue         CleanupExitKind = "continue"
	CleanupExitReturn           CleanupExitKind = "return"
	CleanupExitErrorPropagation CleanupExitKind = "error-propagation"
	CleanupExitPanic            CleanupExitKind = "panic"
)

// ResolvedCleanupExit is one exit edge: the scopes it leaves (innermost
// first), whether it is a function exit that also runs the remaining
// function-scoped registrations, whether it runs cleanup at all, and the
// source of the protected value whose ownership leaves first.
type ResolvedCleanupExit struct {
	Kind         CleanupExitKind
	Source       lexer.Token
	Invocation   CallableID
	Scope        CleanupScopeID
	Exited       []CleanupScopeID
	FunctionExit bool
	RunsCleanup  bool
	// Protected is the returned or propagated value's source (zero when
	// nothing is protected); ProtectedKind is "return-value" or "error".
	Protected     lexer.Token
	ProtectedKind string
}

// cleanupScopeFacts are the recorded facts of one lexical cleanup scope.
type cleanupScopeFacts struct {
	Parent     CleanupScopeID
	Invocation CallableID
	Source     lexer.Token
	InLoop     bool
	Defer      bool
}

type cleanupExitKey struct {
	Site sourceTokenKey
	Kind CleanupExitKind
}

// cleanupTrySite is where a try expression was analyzed; whether it
// propagates is read from its resolved kind when the plan is built.
type cleanupTrySite struct {
	Scope      CleanupScopeID
	Invocation CallableID
}

// cleanupFlowState is the per-analysis cleanup flow record.
type cleanupFlowState struct {
	scopes      map[CleanupScopeID]cleanupScopeFacts
	roots       map[CallableID]CleanupScopeID
	loopEntries []CleanupScopeID
	exits       []ResolvedCleanupExit
	exitKeys    map[cleanupExitKey]bool
	trySites    map[*ast.TryExpression]cleanupTrySite
}

// recordCleanupScope retains a scope's parent, invocation and loop context;
// the first scope of an invocation is its function root.
func (a *Analyzer) recordCleanupScope(id, parent CleanupScopeID, block *ast.BlockStatement) {
	if id == 0 {
		return
	}
	flow := &a.cleanupFlow
	if flow.scopes == nil {
		flow.scopes = map[CleanupScopeID]cleanupScopeFacts{}
		flow.roots = map[CallableID]CleanupScopeID{}
	}
	flow.scopes[id] = cleanupScopeFacts{Parent: parent, Invocation: a.currentCallable, Source: block.Token, InLoop: a.loopDepth > 0, Defer: a.inDeferBlock}
	if _, rooted := flow.roots[a.currentCallable]; !rooted && !a.inDeferBlock {
		flow.roots[a.currentCallable] = id
	}
}

// recordCleanupBlockExit records falling off the end of a block that can
// fall through; falling off the function root is a function exit.
func (a *Analyzer) recordCleanupBlockExit(id CleanupScopeID, block *ast.BlockStatement) {
	if id == 0 || a.inDeferBlock || !a.blockCanFallThrough(block) {
		return
	}
	a.recordCleanupExit(CleanupExitFallthrough, block.Token, id, []CleanupScopeID{id}, a.cleanupFlow.roots[a.currentCallable] == id, nil, "")
}

// enterCleanupLoop marks the scope a loop's break/continue return to.
func (a *Analyzer) enterCleanupLoop() func() {
	a.cleanupFlow.loopEntries = append(a.cleanupFlow.loopEntries, a.currentCleanupScope)
	return func() { a.cleanupFlow.loopEntries = a.cleanupFlow.loopEntries[:len(a.cleanupFlow.loopEntries)-1] }
}

// recordCleanupJump records break/continue: it leaves every scope up to the
// innermost loop's entry scope.
func (a *Analyzer) recordCleanupJump(kind CleanupExitKind, token lexer.Token) {
	if len(a.cleanupFlow.loopEntries) == 0 || a.inDeferBlock {
		return
	}
	entry := a.cleanupFlow.loopEntries[len(a.cleanupFlow.loopEntries)-1]
	a.recordCleanupExit(kind, token, a.currentCleanupScope, a.cleanupScopeChain(a.currentCleanupScope, entry), false, nil, "")
}

// recordCleanupReturn records a return: it leaves every scope of the
// invocation, protecting the returned value.
func (a *Analyzer) recordCleanupReturn(stmt *ast.ReturnStatement) {
	if a.inDeferBlock {
		return
	}
	var protected ast.Expression
	if stmt.Value != nil {
		protected = stmt.Value
	}
	a.recordCleanupExit(CleanupExitReturn, stmt.Token, a.currentCleanupScope, a.cleanupScopeChain(a.currentCleanupScope, 0), true, protected, "return-value")
}

// recordCleanupPanic records a non-unwinding exit, which runs no cleanup.
func (a *Analyzer) recordCleanupPanic(token lexer.Token) {
	if a.inDeferBlock {
		return
	}
	a.recordCleanupExit(CleanupExitPanic, token, a.currentCleanupScope, nil, false, nil, "")
}

// recordCleanupTrySite retains where a try was analyzed.
func (a *Analyzer) recordCleanupTrySite(expr *ast.TryExpression) {
	if a.inDeferBlock || a.summaryPass {
		return
	}
	if a.cleanupFlow.trySites == nil {
		a.cleanupFlow.trySites = map[*ast.TryExpression]cleanupTrySite{}
	}
	a.cleanupFlow.trySites[expr] = cleanupTrySite{Scope: a.currentCleanupScope, Invocation: a.currentCallable}
}

func (a *Analyzer) recordCleanupExit(kind CleanupExitKind, token lexer.Token, scope CleanupScopeID, exited []CleanupScopeID, functionExit bool, protected ast.Expression, protectedKind string) {
	if a.summaryPass || scope == 0 || !validDefinitionToken(token) {
		return
	}
	flow := &a.cleanupFlow
	key := cleanupExitKey{Site: sourceTokenLocation(token), Kind: kind}
	if flow.exitKeys == nil {
		flow.exitKeys = map[cleanupExitKey]bool{}
	}
	if flow.exitKeys[key] {
		return
	}
	flow.exitKeys[key] = true
	exit := ResolvedCleanupExit{Kind: kind, Source: token, Invocation: a.currentCallable, Scope: scope,
		Exited: exited, FunctionExit: functionExit, RunsCleanup: kind != CleanupExitPanic}
	if protected != nil {
		exit.Protected, exit.ProtectedKind = expressionToken(protected), protectedKind
	}
	flow.exits = append(flow.exits, exit)
}

// cleanupScopeChain lists the scopes from scope outward, stopping before
// stop (zero: up to and including the invocation's root).
func (a *Analyzer) cleanupScopeChain(scope, stop CleanupScopeID) []CleanupScopeID {
	var chain []CleanupScopeID
	invocation := a.cleanupFlow.scopes[scope].Invocation
	for scope != 0 && scope != stop {
		facts, known := a.cleanupFlow.scopes[scope]
		if !known || facts.Invocation != invocation {
			break
		}
		chain = append(chain, scope)
		if scope == a.cleanupFlow.roots[invocation] {
			break
		}
		scope = facts.Parent
	}
	return chain
}
