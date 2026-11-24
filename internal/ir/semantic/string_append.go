package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildStringAppend reads the destination once, prepares one maximal string,
// and commits only on materialization success. A protected append routes every
// represented RHS failure and its own StringError before any store; handlers
// run with the original destination. Property accessors and unrepresented
// failure kinds remain named boundaries rather than approximated writes.
// Rules: rules/foundations/operators.md — §§29.3,29.9;
// rules/errors/errorhandling.md — §§23,35; MD-039 §§3–4.
func (fb *functionBuilder) buildStringAppend(stmt *ast.AssignmentStatement, protected *ast.TryAssignmentStatement) error {
	resolved, ok := fb.owner.analyzer.ResolvedStringAppendOf(stmt.Value)
	if !ok || stmt.Operator != "+=" || fb.owner.maxPackage < arenaLoweringPackage {
		return fb.unsupported("string append before explicit allocation-context IR", stmt.Token)
	}
	if rhs, known := fb.owner.analyzer.StringConcatPlanOf(stmt.Value); known && rhs.Runtime && rhs.Interpolated {
		return fb.unsupported("string append over runtime interpolation", stmt.Token)
	}
	var fact sema.ResolvedTryAssignment
	var router *failureRouter
	point := sema.TryFailurePoint{Kind: sema.TryFailureAllocation, ErrorType: resolved.Plan.Allocation.FailureType, Expression: stmt.Value, Token: expressionToken(stmt.Value)}
	if protected != nil {
		fact, ok = fb.owner.analyzer.ResolvedTryAssignmentOf(protected)
		if !ok || resolved.Plan.Failure != sema.StringMaterializationTry {
			return fmt.Errorf("string append lacks its resolved try assignment")
		}
		router = &failureRouter{points: map[ast.Expression]sema.TryFailurePoint{stmt.Value: point}, routed: map[ast.Expression]bool{}}
		expectedError, err := fb.owner.internType(fact.ErrorType)
		if err != nil {
			return err
		}
		for _, failure := range fact.Failures {
			failureError, err := fb.owner.internType(failure.ErrorType)
			if err != nil {
				return err
			}
			if failure.Kind != sema.TryFailureAllocation || failureError != expectedError {
				return fb.unsupported("string append with an unrepresented RHS failure "+string(failure.Kind), failure.Token)
			}
			router.points[failure.Expression] = failure
		}
	} else if resolved.Plan.Failure != sema.StringMaterializationPanic {
		return fmt.Errorf("ordinary string append lacks its panic mode")
	}
	current, commit, err := fb.stringAppendDestination(stmt.Target)
	if err != nil {
		return err
	}
	if !typeHasKind(fb.owner.module.Types, current.typ, TypeString) {
		return fb.unsupported("string append to a constrained or non-string destination", stmt.Token)
	}
	previous := fb.failures
	if router != nil {
		fb.failures = router
	}
	updated, err := fb.buildPlannedStringMaterialization(stmt.Value, current.typ, resolved.Plan, []builtValue{current}, location(stmt.Token))
	fb.failures = previous // handler bodies are outside the protected subtree
	if err != nil {
		return err
	}
	if router != nil {
		for expression := range router.points {
			if !router.routed[expression] {
				return fb.unsupported("string append RHS failure without a route", expressionToken(expression))
			}
		}
	}
	commit(updated)
	if protected == nil {
		return nil
	}
	return fb.finishStringAppendTry(protected, fact, router)
}

// stringAppendDestination produces an already-read destination and its one
// commit operation. Binding/capture/static and checked reference roots reuse
// their canonical Place producers; property setters require their own protocol.
// Rules: rules/foundations/operators.md — §29.9(2);
// rules/compiler/semantic_ir.md — §§26,31; rules/memory/references.md — explicit dereference.
func (fb *functionBuilder) stringAppendDestination(target ast.Expression) (builtValue, func(builtValue), error) {
	loc := locationFromExpression(target)
	if id, ok := target.(*ast.Identifier); ok {
		if global, isStatic, err := fb.staticOfIdentifier(id); isStatic {
			if err != nil {
				return builtValue{}, nil, err
			}
			current := fb.result(Operation{Kind: OpStaticLoad, Static: global.ID, Location: loc}, global.Type)
			return current, func(value builtValue) {
				fb.emit(Operation{Kind: OpStaticStore, Static: global.ID, Operands: []ValueID{value.id}, Location: loc})
			}, nil
		}
		place, mutable, err := fb.bindingRootPlace(id)
		if err != nil {
			return builtValue{}, nil, err
		}
		if !mutable {
			return builtValue{}, nil, fb.unsupported("string append to an immutable Place", id.Token)
		}
		typ, _ := fb.owner.module.Types.Lookup(place.typ)
		current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, typ.Element)
		return current, func(value builtValue) {
			fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: loc})
		}, nil
	}
	if use, ok := fb.owner.referenceUseOf(target); ok && use.Kind == sema.ReferenceUseWrite && use.Facts.Checkable() {
		place, mutable, err := fb.derefUsePlace(target, use)
		if err != nil {
			return builtValue{}, nil, err
		}
		if !mutable {
			return builtValue{}, nil, fb.unsupported("string append through a shared reference", expressionToken(target))
		}
		typ, _ := fb.owner.module.Types.Lookup(place.typ)
		current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, typ.Element)
		return current, func(value builtValue) {
			fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: loc})
		}, nil
	}
	// Stored field and proven fixed-array projections reuse Sema's canonical
	// Place path. Properties require accessors; runtime destination checks
	// require their own protected failure facts and cannot become hidden panics.
	var steps []ast.Expression
	root := target
	for {
		switch step := root.(type) {
		case *ast.MemberExpression:
			steps = append([]ast.Expression{step}, steps...)
			root = step.Object
			continue
		case *ast.IndexExpression:
			plan, known := fb.owner.analyzer.ResolvedArrayIndexPlanOf(step)
			if !known || plan.CheckKind != sema.ArrayIndexProvenSafe {
				return builtValue{}, nil, fb.unsupported("runtime-checked string append destination", step.Token)
			}
			steps = append([]ast.Expression{step}, steps...)
			root = step.Left
			continue
		}
		break
	}
	if id, named := root.(*ast.Identifier); named && len(steps) > 0 {
		resolved, known := fb.owner.analyzer.ResolvedPlaceOf(target)
		if !known || len(resolved.Projections) != len(steps) {
			return builtValue{}, nil, fb.unsupported("string append without a resolved stored-field Place", expressionToken(target))
		}
		place, mutable, err := fb.bindingRootPlace(id)
		if err != nil {
			return builtValue{}, nil, err
		}
		if !mutable {
			return builtValue{}, nil, fb.unsupported("string append to immutable field storage", expressionToken(target))
		}
		place, err = fb.projectPlace(steps, resolved.Projections, place, mutable, placeMetadataOf(resolved))
		if err != nil {
			return builtValue{}, nil, err
		}
		typ, _ := fb.owner.module.Types.Lookup(place.typ)
		current := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: loc}, typ.Element)
		return current, func(value builtValue) {
			fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: loc})
		}, nil
	}
	return builtValue{}, nil, fb.unsupported("string append to a projected/property Place", expressionToken(target))
}

// finishStringAppendTry joins the already-committed success path with local
// void handlers, or propagates the exact StringError from every failure edge.
// Rules: rules/errors/errorhandling.md — §§15–16,23,35; MD-039 §4(2).
func (fb *functionBuilder) finishStringAppendTry(stmt *ast.TryAssignmentStatement, fact sema.ResolvedTryAssignment, router *failureRouter) error {
	if len(router.targets) != 1 {
		return fb.unsupported("heterogeneous string append failures", stmt.Token)
	}
	target := router.targets[0]
	success := fb.current
	if fact.Kind == sema.ResolvedTryAssignmentHandled {
		if !lowerableTryPlan(fact.HandlerPlan) {
			return fb.unsupported("unrepresented string append handler plan", stmt.Token)
		}
		voidType, err := fb.owner.internType(sema.Type{Name: "void", Kind: sema.VoidType})
		if err != nil {
			return err
		}
		handlers := &ast.TryExpression{Token: stmt.Token, Handlers: stmt.Handlers}
		_, err = fb.buildLocalTryHandlers(handlers, fact.HandlerPlan, success, builtValue{typ: voidType}, target.block, builtValue{id: target.value, typ: target.errorType})
		return err
	}
	enclosing, err := fb.owner.internType(fact.EnclosingResultType)
	if err != nil {
		return err
	}
	typ, ok := fb.owner.module.Types.Lookup(enclosing)
	if !ok || typ.Kind != TypeResult || typ.Error != target.errorType {
		return fb.unsupported("string append propagation with error widening", stmt.Token)
	}
	fb.current = target.block
	failure := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{target.value}, Location: location(stmt.Token)}, enclosing)
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{failure.id}, Location: location(stmt.Token)})
	fb.current = success
	return nil
}
