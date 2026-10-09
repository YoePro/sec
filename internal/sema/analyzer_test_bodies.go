package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzeTestBodies applies ordinary Sec statement, scope, ownership, and
// cleanup analysis to every retained test body while preserving the test
// declaration itself as the semantic context without a source-callable name.
//
// Rules:
//   - rules/tooling/testing.md — §9.1 "Ordinary body semantics"
//   - rules/tooling/testing.md — §§9.2–9.4 test completion and return
//   - rules/tooling/testing.md — §21 "Cleanup and controlled termination"
func (a *Analyzer) analyzeTestBodies(program *ast.Program) {
	a.withProgramModules(program, func(statement ast.Statement) {
		declaration, ok := statement.(*ast.TestDeclaration)
		if !ok || declaration == nil || declaration.Body == nil {
			return
		}
		a.analyzeTestBody(declaration)
	})
}

// analyzeTestBody establishes an invocation-local Sema scope for a source
// test. It intentionally creates neither a Function nor a source-callable name.
//
// Rules:
//   - rules/analysis/call_graph.md — "Test roots", "Callable node"
//   - rules/tooling/testing.md — §5.7 "Not callable as an ordinary function"
//   - rules/tooling/testing.md — §9 "Test body execution"
func (a *Analyzer) analyzeTestBody(declaration *ast.TestDeclaration) {
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousFunctionName := a.currentFunctionName
	previousCallable := a.currentCallable
	previousFunctionReturn := a.currentFunctionReturn
	previousFunctionToken := a.currentFunctionToken
	previousFunctionMetadata := a.currentFunctionMetadata
	previousFunctionSummary := a.currentFunctionSummary
	previousHasFunctionSummary := a.hasCurrentFunctionSummary
	previousInFunctionBody := a.inFunctionBody
	previousTest := a.currentTest
	previousScopeDepth := a.scopeDepth

	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = map[string]lexer.Token{}
	a.moveReasons = map[string]string{}
	a.closedResources = map[string]lexer.Token{}
	a.borrows = map[string][]borrowRecord{}
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.currentFunctionName = ""
	a.currentCallable = a.recordTestCallable(declaration)
	a.currentFunctionReturn = Type{Name: "void", Kind: VoidType}
	a.currentFunctionToken = declaration.Token
	a.currentFunctionMetadata = Function{}
	a.currentFunctionSummary = localReferenceOrigin{}
	a.hasCurrentFunctionSummary = false
	a.inFunctionBody = true
	a.currentTest = declaration
	a.scopeDepth = 0
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.currentFunctionName = previousFunctionName
		a.currentCallable = previousCallable
		a.currentFunctionReturn = previousFunctionReturn
		a.currentFunctionToken = previousFunctionToken
		a.currentFunctionMetadata = previousFunctionMetadata
		a.currentFunctionSummary = previousFunctionSummary
		a.hasCurrentFunctionSummary = previousHasFunctionSummary
		a.inFunctionBody = previousInFunctionBody
		a.currentTest = previousTest
		a.scopeDepth = previousScopeDepth
	}()

	a.analyzeBlockStatements(declaration.Body)
}

// tryPropagatesToTestBoundary reports whether a bodyless try (or try
// assignment) at the current position propagates its Err to the enclosing
// test invocation. A test body is a compiler-known propagation boundary: any
// error type fails the current invocation after ordinary cleanup, without a
// source-visible Result. A lambda inside the test is its own function and
// clears currentTest; defer bodies are rejected before this point.
//
// Rules:
//   - rules/errors/errorhandling.md — §41 "Test propagation boundary"
//   - rules/tooling/testing.md — §10.1 "Test boundary supports try", §10.2 "Unexpected propagated error"
func (a *Analyzer) tryPropagatesToTestBoundary() bool {
	return a.currentTest != nil && a.inFunctionBody && !a.inDeferBlock
}
