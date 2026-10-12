package semantic

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// failureRouter is active while the protected expression of a failure-set try
// is built. Each Sema-recorded failure point that fails branches to the
// failure edge of its error type with the typed error value, so the first
// failing point in evaluation order leaves the protected expression and later
// points are never evaluated.
type failureRouter struct {
	points  map[ast.Expression]sema.TryFailurePoint
	routed  map[ast.Expression]bool
	targets []*failureTarget
}

// failureTarget is the failure edge of one member error type of the set.
type failureTarget struct {
	errorType TypeID
	block     *Block
	value     ValueID
}

// failurePoint reports whether expr is a failure point of the active
// failure-set try.
func (fb *functionBuilder) failurePoint(expr ast.Expression) (sema.TryFailurePoint, bool) {
	if fb.failures == nil || expr == nil {
		return sema.TryFailurePoint{}, false
	}
	point, ok := fb.failures.points[expr]
	return point, ok
}

// routeFailure branches from the current block to the failure edge of the
// error value's type and terminates the current block.
func (fb *functionBuilder) routeFailure(expr ast.Expression, errorValue builtValue, loc Location) {
	router := fb.failures
	var target *failureTarget
	for _, candidate := range router.targets {
		if candidate.errorType == errorValue.typ {
			target = candidate
		}
	}
	if target == nil {
		block := fb.newBlock()
		parameter := fb.newValue(errorValue.typ, OwnershipImmediate, loc)
		block.Parameters = []Value{parameter}
		target = &failureTarget{errorType: errorValue.typ, block: block, value: parameter.ID}
		router.targets = append(router.targets, target)
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: target.block.ID, Arguments: []ValueID{errorValue.id}}}, Location: loc})
	fb.current = nil
	router.routed[expr] = true
}

// outOfBoundsError materializes IndexError.OutOfBounds of the resolved index
// error type.
func (fb *functionBuilder) outOfBoundsError(errorType sema.Type, token lexer.Token) (builtValue, error) {
	typ, err := fb.owner.internType(errorType)
	if err != nil {
		return builtValue{}, err
	}
	definition, ok := fb.owner.enumDefinition(typ)
	if !ok {
		return builtValue{}, fb.unsupported("IndexError enum definition", token)
	}
	caseID, ok := enumCaseIDByName(definition, "OutOfBounds")
	if !ok {
		return builtValue{}, fb.unsupported("IndexError.OutOfBounds", token)
	}
	return fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, Location: location(token)}, typ), nil
}

// buildFailureSetTry lowers a try whose protected expression contains an
// ordered compiler-internal failure set. The expression is built once in
// evaluation order with every recorded checked operator and fixed-array index
// routed to a typed failure edge, and a Result carrier root tested last; the
// success value continues. Each failure edge then propagates through the
// enclosing Result or dispatches the local handlers. A point the builder
// cannot route (dynamic or list indexing, contract conversion, string
// allocation, checked conversion) and a heterogeneous set, which needs error
// widening, are rejected rather than lowered as panics.
//
// Rules:
//   - rules/errors/errorhandling.md — §11 "Compiler-internal failure sets", §§15-16, §35
//   - rules/errors/runtime_checks.md — "What try converts", "Propagation rule"
func (fb *functionBuilder) buildFailureSetTry(expr *ast.TryExpression, resolved sema.ResolvedTry) (builtValue, error) {
	if resolved.TestBoundary {
		return builtValue{}, fb.unsupported("try propagation to a test boundary", expr.Token)
	}
	var plan sema.ResolvedTryPlan
	if resolved.Kind == sema.ResolvedTryHandledFailureSet {
		var ok bool
		plan, ok = fb.owner.analyzer.ResolvedTryPlanOf(expr)
		if !ok || !lowerableTryPlan(plan) {
			return builtValue{}, fb.unsupported("unresolved failure-set try handler plan", expr.Token)
		}
		if plan.ResidualTestBoundary {
			return builtValue{}, fb.unsupported("try residual propagation to a test boundary", expr.Token)
		}
	}
	router := &failureRouter{points: map[ast.Expression]sema.TryFailurePoint{}, routed: map[ast.Expression]bool{}}
	var carrier *sema.TryFailurePoint
	for index, point := range resolved.Failures {
		switch point.Kind {
		case sema.TryFailureArithmetic, sema.TryFailureBounds:
			router.points[point.Expression] = point
		case sema.TryFailureCarrier:
			if point.Expression != expr.Expression {
				// An intermediate Result receiver of a protected chain needs
				// its success value routed into the next member call.
				return builtValue{}, fb.unsupported("failure-set try over an intermediate Result receiver of a call chain", point.Token)
			}
			carrier = &resolved.Failures[index]
		default:
			return builtValue{}, fb.unsupported("failure-set try over a "+string(point.Kind)+" failure", point.Token)
		}
	}
	successType, err := fb.owner.internType(resolved.SuccessType)
	if err != nil {
		return builtValue{}, err
	}
	previous := fb.failures
	fb.failures = router
	var success builtValue
	if carrier != nil {
		success, err = fb.buildRoutedCarrier(expr, *carrier)
	} else {
		success, err = fb.buildExpr(expr.Expression, successType)
	}
	fb.failures = previous
	if err != nil {
		return builtValue{}, err
	}
	for point := range router.points {
		if !router.routed[point] {
			return builtValue{}, fb.unsupported("failure-set try point that the protected expression lowering does not route", expressionToken(point))
		}
	}
	if fb.current == nil {
		return builtValue{}, fb.unsupported("failure-set try without a success continuation", expr.Token)
	}

	if resolved.Kind == sema.ResolvedTryHandledFailureSet {
		if len(router.targets) != 1 {
			return builtValue{}, fb.unsupported("handlers over a heterogeneous failure set (error widening)", expr.Token)
		}
		target := router.targets[0]
		return fb.buildLocalTryHandlers(expr, plan, fb.current, success, target.block, builtValue{id: target.value, typ: target.errorType})
	}
	enclosing, err := fb.owner.internType(resolved.EnclosingResultType)
	if err != nil {
		return builtValue{}, err
	}
	enclosingType, ok := fb.owner.module.Types.Lookup(enclosing)
	if !ok || enclosingType.Kind != TypeResult {
		return builtValue{}, fb.unsupported("failure-set propagation without an enclosing Result", expr.Token)
	}
	continuation := fb.current
	for _, target := range router.targets {
		if target.errorType != enclosingType.Error {
			return builtValue{}, fb.unsupported("failure-set propagation with error widening", expr.Token)
		}
		fb.current = target.block
		propagated := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{target.value}, Location: location(expr.Token)}, enclosing)
		fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{propagated.id}, Location: location(expr.Token)})
	}
	fb.current = continuation
	return success, nil
}

// buildRoutedCarrier builds a Result carrier root whose operands may contain
// routed failure points, then routes its Err state as the last failure point.
func (fb *functionBuilder) buildRoutedCarrier(expr *ast.TryExpression, point sema.TryFailurePoint) (builtValue, error) {
	resultValue, err := fb.buildExpr(expr.Expression, 0)
	if err != nil {
		return builtValue{}, err
	}
	resultType, ok := fb.owner.module.Types.Lookup(resultValue.typ)
	if !ok || resultType.Kind != TypeResult {
		return builtValue{}, fb.unsupported("failure-set carrier without Semantic IR Result type", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, boolType)
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: location(expr.Token)})
	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpResultUnwrapErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Error)
	fb.failures.points[point.Expression] = point
	fb.routeFailure(point.Expression, errorValue, location(expr.Token))
	fb.current = successBlock
	if fb.owner.isVoidType(resultType.Success) {
		return builtValue{typ: resultType.Success}, nil
	}
	return fb.result(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Success), nil
}
