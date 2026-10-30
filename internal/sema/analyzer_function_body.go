package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzeFunctionBodyInScope checks one callable in an isolated body scope,
// restores its caller's analysis state, and records reference summaries.
// Unsafe callable status never makes the body implicitly unsafe.
// Rules: rules/declarations/functions.md — parameter binding and function bodies;
// rules/memory/unsafe.md — §7(1–5);
// rules/memory/raw_pointers.md — §6(2–3).
func (a *Analyzer) analyzeFunctionBodyInScope(fn *ast.FunctionDeclaration, name string) {
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	if a.summaryPass && !typeContainsReference(function.ReturnType, map[string]bool{}) && !hasArenaReferenceParameter(function) {
		return
	}
	if function.Extern {
		a.validateExternFunction(function, fn)
		if fn.Body == nil {
			return
		}
		a.addErrorAtToken(fn.Token, "extern function declarations may not have a Sec body")
		return
	}

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
	previousUnsafe := a.inUnsafe
	previousScopeDepth := a.scopeDepth
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = map[string]lexer.Token{}
	a.moveReasons = map[string]string{}
	a.closedResources = map[string]lexer.Token{}
	a.borrows = a.staticHolderBorrows()
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.scopeDepth = 0
	a.currentFunctionName = fn.Name.Value
	previousStableBindings := a.currentStableBindings
	defer func() { a.currentStableBindings = previousStableBindings }()
	a.currentStableBindings = stableBindingsOf(fn)
	previousLoopWritten := a.loopWrittenNames
	defer func() { a.loopWrittenNames = previousLoopWritten }()
	a.loopWrittenNames = loopAssignedNames(fn)
	a.currentCallable = a.callGraph.addCallable(function)
	a.currentFunctionReturn = function.ReturnType
	a.currentFunctionToken = function.Token
	a.currentFunctionMetadata = function
	a.currentFunctionSummary = localReferenceOrigin{}
	a.hasCurrentFunctionSummary = false
	previousArenaParameterResets := a.currentArenaParameterResets
	a.currentArenaParameterResets = map[int]bool{}
	defer func() { a.currentArenaParameterResets = previousArenaParameterResets }()
	a.inFunctionBody = true
	// Callable safety constrains callers; it grants no lexical permission
	// inside this body, nor does an enclosing analysis context.
	a.inUnsafe = false
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.borrows = previousBorrows
		a.closedResources = previousClosedResources
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
		a.inUnsafe = previousUnsafe
		a.scopeDepth = previousScopeDepth
	}()

	for _, param := range function.Parameters {
		// An owned by-value parameter is the callee's mutable working binding.
		// Reference parameters retain their referent authority in the type, but
		// the reference binding itself is not implicitly rebindable.
		bindingType := param.Type
		mutableBinding := !param.Ref && !param.MutableRef && param.Type.Kind != ReferenceType
		if param.Variadic {
			// rules/declarations/functions.md sections 29-31: the callee sees
			// one ephemeral, structurally read-only pack, never a mutable array.
			bindingType = NewVariadicPackType(param.Type)
			mutableBinding = false
		}
		symbol := Symbol{Name: param.Name, Type: bindingType, Mutable: mutableBinding, Token: param.Token, Storage: StorageOriginAutomatic, Local: true, Parameter: true, ScopeDepth: 0}
		a.symbols[param.Name] = symbol
		a.seedParameterReferenceOrigin(function, param)
		symbol = a.symbols[param.Name]
		completionSymbol := symbol
		completionSymbol.Local = true
		a.completionSymbols[param.Name] = completionSymbol
		delete(a.constInts, param.Name)
		a.assigned[param.Name] = true
		a.recordDefinition(param.Token)
		a.recordBinding(param.Token, BindingParameter, param.Name, symbol.Type, mutableBinding)
		a.recordReferenceParameterFacts(param)
	}
	// rules/declarations/static.md, receiver semantics; correction15.md keeps the
	// owning type on static methods but does not synthesize self or members.
	if !function.Static {
		a.defineImplicitImplInstanceSymbols(fn.Body, function.ReceiverMutable)
		a.recordReceiverFacts(function)
	}

	a.analyzeBlockStatements(fn.Body)
	a.setFunctionReferenceSummary(function)

	if !function.Initializer && !a.blockDefinitelyReturns(fn.Body) && function.ReturnType.Kind != VoidType && function.ReturnType.Kind != NeverType {
		a.addErrorAtToken(fn.Name.Token, "function %s must return %s", fn.Name.Value, typeDisplayName(function.ReturnType))
	}
}
