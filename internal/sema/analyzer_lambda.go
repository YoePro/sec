package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inferLambdaExpression validates a lambda, constructs its capture environment
// in the enclosing callable, and analyzes its body under a distinct callable
// identity and lexical capture scope.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§13–21 explicit captures and lambda bodies
//   - rules/analysis/closure_analysis.md — "Callable creation"
//   - rules/analysis/call_graph.md — "Callable node" and "Call-site record"
//   - rules/control-flow/defer.md — §§3,16–18 independent invocation boundaries
func (a *Analyzer) inferLambdaExpression(expr *ast.LambdaExpression) (Type, expressionValue) {
	returnType, ok := a.resolveType(expr.ReturnType)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	params := make([]Type, 0, len(expr.Parameters))
	seenParams := map[string]lexer.Token{}
	for _, param := range expr.Parameters {
		if _, exists := seenParams[param.Name.Value]; exists {
			a.addErrorAtToken(param.Name.Token, "duplicate parameter %q", param.Name.Value)
			continue
		}
		a.checkConfusableInDomains(param.Name.Value, param.Name.Token, seenParams)
		seenParams[param.Name.Value] = param.Name.Token

		paramType, paramOK := a.resolveType(param.Type)
		if !paramOK {
			continue
		}
		params = append(params, paramType)
	}

	lambdaType := Type{
		Name:                   functionTypeName(params, returnType, CallableShared),
		Kind:                   FunctionType,
		FunctionParameterTypes: params,
		FunctionReturnType:     &returnType,
		FunctionCapability:     CallableShared,
	}
	if len(params) != len(expr.Parameters) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	a.validateLambdaCaptureOwnership(expr)
	a.recordLambdaCaptureFacts(expr)
	a.recordLambdaCallableIdentity(expr)
	lambdaCallable := CallableID("")
	if identity, exists := a.resolvedCallableIdentities[expr]; exists {
		lambdaCallable = a.callGraph.addClosureCallable(identity, a.currentModule)
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.availabilityStates
	previousFunctionName := a.currentFunctionName
	previousCallable := a.currentCallable
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousInLambda := a.inLambda
	previousLambdaOuterSymbols := a.lambdaOuterSymbols
	previousLoopDepth := a.loopDepth
	previousTest := a.currentTest
	previousEnvironment := a.lambdaEnvironment
	previousBorrows := a.borrows
	environment := &lambdaEnvironmentState{owned: map[string]bool{}}

	// rules/declarations/lambda-functions.md, capture eligibility; correction11.md
	// separates enclosing locals from ordinary module/type lookup. Non-local
	// declarations remain directly visible and cannot be captured.
	captureCandidates := map[string]Symbol{}
	// A child layer hides the enclosing locals instead of copying every
	// module-level symbol (symbolTable).
	a.symbols = previousSymbols.child()
	a.constInts = map[string]*big.Int{}
	a.assigned = map[string]bool{}
	a.moved = copyMoved(previousMoved)
	a.availabilityStates = copyAvailabilityStates(previousMoveReasons)
	for _, capture := range expr.Captures {
		if capture.Name != nil {
			clearRootPlaceStateMaps(a.moved, a.availabilityStates, capture.Name.Value)
		}
	}
	for name, symbol := range previousSymbols.all() {
		if symbol.Local {
			captureCandidates[name] = symbol
			a.symbols.del(name)
			continue
		}
		a.assigned[name] = previousAssigned[name]
		if value, ok := previousConstInts[name]; ok {
			a.constInts[name] = new(big.Int).Set(value)
		}
	}
	a.currentFunctionName = "lambda"
	a.currentFunctionReturn = returnType
	a.inFunctionBody = true
	previousTryHandlerDepth := a.tryHandlerDepth
	a.tryHandlerDepth = 0
	a.inLambda = true
	a.lambdaOuterSymbols = captureCandidates
	a.loopDepth = 0
	// A lambda declared inside a test is its own callable boundary. Its return
	// statements follow the declared lambda return type, not test-return rules.
	a.currentTest = nil
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.availabilityStates = previousMoveReasons
		a.currentFunctionName = previousFunctionName
		a.currentCallable = previousCallable
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.tryHandlerDepth = previousTryHandlerDepth
		a.inLambda = previousInLambda
		a.lambdaOuterSymbols = previousLambdaOuterSymbols
		a.loopDepth = previousLoopDepth
		a.currentTest = previousTest
		a.lambdaEnvironment = previousEnvironment
		a.borrows = previousBorrows
	}()

	a.lambdaEnvironment = environment
	// A capture name inside the body denotes the environment field or the
	// borrow taken at construction, not the outer Place: outer borrows of
	// that root were checked when the closure was built (§23).
	a.borrows = copyBorrows(previousBorrows)
	for _, capture := range expr.Captures {
		if capture.Name != nil {
			delete(a.borrows, capture.Name.Value)
		}
	}
	a.defineLambdaCaptures(expr, captureCandidates, previousAssigned)
	defer a.enterIndependentDeferScope()()
	// Environment construction executes in the enclosing callable. Only the
	// lambda body itself changes the active call-graph caller.
	a.currentCallable = lambdaCallable

	for i, param := range expr.Parameters {
		mutableBinding := !param.Ref && !param.MutableRef && params[i].Kind != ReferenceType
		if !a.defineSymbol(param.Name.Value, params[i], mutableBinding, param.Name.Token) {
			continue
		}
		a.assigned[param.Name.Value] = true
	}

	a.analyzeBlockStatements(expr.Body)

	if !a.blockDefinitelyReturns(expr.Body) && returnType.Kind != VoidType {
		a.addErrorAtToken(expr.Token, "lambda must return %s", typeDisplayName(returnType))
	}

	// §§9, 20, 27–28: the callable capability and the closure's
	// move-only environment follow from how the body used its environment.
	capability := environment.inferredCapability()
	lambdaType.FunctionCapability = capability
	lambdaType.Name = functionTypeName(params, returnType, capability)
	lambdaType.FunctionEnvironmentMoveOnly = environment.environmentMoveOnly()
	return lambdaType, expressionValue{Display: expr.String()}
}
