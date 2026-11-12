package sema

import "sec/internal/ast"

// inferSpawnExpression validates the child callable/result and execution-bound
// guard dependencies before committing ownership across the execution boundary.
// Rules: rules/concurrency/spawn.md — "Result type", "Ownership transfer", "Captures and callable values";
// rules/concurrency/tasks.md — §4(1–7); rules/concurrency/mutex.md — §§39–40;
// rules/memory/transferability.md — §§15,17–18.
func (a *Analyzer) inferSpawnExpression(expr *ast.SpawnExpression) (Type, expressionValue) {
	if expr.Body != nil {
		// rules/concurrency/spawn.md §2(2–3), grammar.md §89: block syntax
		// is canonical even while its capture/child-body lowering is absent.
		a.reportCapabilityUnavailable(expr.Token,
			"This block is valid spawn syntax. Use a callable expression until spawned block bodies and their capture environments are implemented.",
			"compiler capability unavailable: spawned block body for %s", expr.Kind)
		a.withCancellableContext(func() {
			a.analyzeBlockStatements(expr.Body)
		})
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if expr.Value == nil {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	// rules/foundations/grammar.md §89; rules/tooling/diagnostics.md —
	// recovery: the parser already diagnosed a missing operand. In an LSP
	// recovery tree it must not become a second callable-shape diagnostic.
	if _, invalid := expr.Value.(*ast.InvalidExpression); invalid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch expr.Value.(type) {
	case *ast.CallExpression, *ast.LambdaExpression:
	default:
		a.addErrorAtToken(expressionToken(expr.Value), "spawn requires a callable expression")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	execution, graphSpawn := spawnExecutionRelation(expr.Kind)
	previousMoved, previousReasons := copyMoved(a.moved), copyAvailabilityStates(a.availabilityStates)
	previousBorrows := copyBorrows(a.borrows)
	configurationCommitted := false
	if expr.Configuration != nil {
		// threads.md §20: invalid source preparation must not leave either
		// the configuration or child arguments spuriously consumed.
		defer func() {
			if !configurationCommitted {
				a.moved, a.availabilityStates, a.borrows = previousMoved, previousReasons, previousBorrows
			}
		}()
	}
	configurationFunction, configurationValid := a.prepareThreadConfiguration(expr)
	if !configurationValid {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	spawnedType, _ := a.inferSpawnValue(expr.Value, execution, graphSpawn)
	if spawnedType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	returnType := spawnedType
	if _, lambda := expr.Value.(*ast.LambdaExpression); lambda {
		// A lambda is the child callable; a call returning a function instead
		// has that function value as its child result (spawn.md, "Result type").
		if spawnedType.Kind != FunctionType || spawnedType.FunctionReturnType == nil {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		returnType = *spawnedType.FunctionReturnType
	}
	if a.checkMutexGuardSpawn(expr, returnType) {
		a.moved, a.availabilityStates, a.borrows = previousMoved, previousReasons, previousBorrows
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	switch expr.Kind {
	case "", "task":
		return a.taskSpawnResultType(returnType), expressionValue{Display: expr.String()}
	case "thread":
		if !a.consumeThreadConfiguration(expr, configurationFunction) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		configurationCommitted = true
		// rules/concurrency/threads.md §§3,26,53: inferred owners retain
		// the same canonical identity as declared Thread[T] parameters.
		return a.intrinsicGenericType("Thread", returnType), expressionValue{Display: expr.String()}
	case "process":
		a.reportCapabilityUnavailable(expr.Token, "Processes are launched through the process library facility until spawn process is implemented.", "spawn process is not implemented yet")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	default:
		a.addErrorAtToken(expr.Token, "unknown spawn kind %s", expr.Kind)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

// inferSpawnValue records the child execution relation while resolving the
// callable in its cancellable execution context, preserving its complete result.
// Rules: rules/concurrency/spawn.md — §§3,16; rules/concurrency/cancellation.md — §§19–23.
func (a *Analyzer) inferSpawnValue(expr ast.Expression, execution CallExecutionRelation, graphSpawn bool) (Type, expressionValue) {
	call, isCall := expr.(*ast.CallExpression)
	if !isCall {
		return a.inferExpressionInCancellableContext(expr)
	}
	previousCall := a.spawnCallExpression
	previousExecution := a.spawnCallExecution
	a.spawnCallExpression = call
	if graphSpawn {
		a.spawnCallExecution = execution
	} else {
		a.spawnCallExecution = ""
	}
	defer func() {
		a.spawnCallExpression = previousCall
		a.spawnCallExecution = previousExecution
	}()
	return a.inferExpressionInCancellableContext(expr)
}

// taskSpawnResultType preserves creation failure outside the complete child
// return type, using the same canonical generic identities as declarations.
// TaskSpawnError is independent of the child TaskOutcome and application E.
// Rules: rules/concurrency/tasks.md — §4(1–7);
// rules/concurrency/spawn.md — §3(1–10).
func (a *Analyzer) taskSpawnResultType(result Type) Type {
	owner := a.intrinsicGenericType("Task", result)
	return a.intrinsicGenericType("Result", owner, a.types["TaskSpawnError"])
}
