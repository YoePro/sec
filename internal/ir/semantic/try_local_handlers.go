package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildLocalTryHandlers dispatches the resolved failure handlers and joins
// recovery values with the implicit success edge. Terminating slice handlers
// preserve their single successful holder without synthesizing reference facts.
// Rules: rules/errors/errorhandling.md — §§15–16,19,21.1,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,60–67,83.
func (fb *functionBuilder) buildLocalTryHandlers(expr *ast.TryExpression, plan sema.ResolvedTryPlan, successBlock *Block, successValue builtValue, errorBlock *Block, errorValue builtValue) (builtValue, error) {
	successType, err := fb.trySuccessType(plan.SuccessType)
	if err != nil {
		return builtValue{}, err
	}
	merge := fb.newBlock()
	var merged builtValue
	if !fb.owner.isVoidType(successType) {
		parameter := fb.newValue(successType, OwnershipImmediate, location(expr.Token))
		merge.Parameters = []Value{parameter}
		merged = builtValue{id: parameter.ID, typ: successType}
	} else {
		merged = builtValue{typ: successType}
	}

	fb.current = successBlock
	// A try handler list contains only alternate/error states. The successful
	// Result path always reaches the merge directly and never dispatches through
	// a source handler.
	//
	// Rules:
	//   - rules/errors/errorhandling.md — §15 "Local try handlers"
	//   - rules/errors/errorhandling.md — §15.1 "Success handlers are forbidden"
	fb.branchToTryMerge(merge, successValue, Operation{TryHandlerKind: TryHandlerOK, TryHandlerIndex: -1, Location: location(expr.Token)})

	fb.current = errorBlock
	residual := plan.ResidualPropagates && !plan.Exhaustive
	for index, handler := range plan.Handlers {
		last := index == len(plan.Handlers)-1
		// A None handler matches the whole alternate state of an Option try,
		// so it dispatches like a catch-all without a variant test.
		coversAlternate := handler.PatternKind == sema.TryHandlerErrCatchAll || handler.PatternKind == sema.TryHandlerOptionNone
		unconditional := !handler.Guarded && (coversAlternate || (last && plan.Exhaustive))
		if coversAlternate {
			// A guarded catch-all tests only its guard; failing it continues
			// with the next handler (errorhandling.md §19).
			var nextTest *Block
			if !unconditional {
				nextTest = fb.newBlock()
			}
			if err := fb.buildTryHandler(expr, handler, errorValue, merge, plan.Exhaustive, residual, nextTest); err != nil {
				return builtValue{}, err
			}
			if unconditional {
				break
			}
			fb.current = nextTest
			continue
		}
		if unconditional {
			if err := fb.buildTryHandler(expr, handler, errorValue, merge, plan.Exhaustive, residual, nil); err != nil {
				return builtValue{}, err
			}
			break
		}
		boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
		if err != nil {
			return builtValue{}, err
		}
		var condition builtValue
		if fb.owner.maxPackage >= 11 {
			definition, ok := fb.owner.enumDefinition(errorValue.typ)
			if !ok {
				return builtValue{}, fb.unsupported("try handler error type without enum definition", expr.Handlers[handler.SourceIndex].Token)
			}
			caseID, ok := enumCaseIDByName(definition, handler.Variant)
			if !ok {
				return builtValue{}, fb.unsupported("unknown resolved enum handler case", expr.Handlers[handler.SourceIndex].Token)
			}
			constant := fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, Location: location(expr.Handlers[handler.SourceIndex].Token)}, errorValue.typ)
			condition = fb.result(Operation{Kind: OpEnumCompare, Operands: []ValueID{errorValue.id, constant.id}, IntegerCompare: IntegerCompareEQ, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, Location: location(expr.Handlers[handler.SourceIndex].Token)}, boolType)
		} else {
			condition = fb.result(Operation{Kind: OpCoreErrorIsVariant, Operands: []ValueID{errorValue.id}, Variant: handler.Variant, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Location: location(expr.Handlers[handler.SourceIndex].Token)}, boolType)
		}
		handlerBlock, nextTest := fb.newBlock(), fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: handlerBlock.ID}, {Block: nextTest.ID}}, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, TryHandlerGuarded: handler.Guarded, Location: location(expr.Handlers[handler.SourceIndex].Token)})
		fb.current = handlerBlock
		var guardFalse *Block
		if handler.Guarded {
			guardFalse = nextTest
		}
		if err := fb.buildTryHandler(expr, handler, errorValue, merge, plan.Exhaustive, residual, guardFalse); err != nil {
			return builtValue{}, err
		}
		fb.current = nextTest
	}
	if fb.current != nil && fb.current != merge && len(fb.current.Operations) == 0 {
		if !residual {
			return builtValue{}, fb.unsupported("non-exhaustive local try handler dispatch", expr.Token)
		}
		// rules/errors/errorhandling.md §16: failures no handler matched leave
		// the function through the enclosing Result channel.
		if err := fb.buildTryResidualPropagation(expr, plan, errorValue); err != nil {
			return builtValue{}, err
		}
	}
	if sourceSliceType(plan.SuccessType) {
		merged, err = fb.completeSliceTryMerge(expr, merge, successValue)
		if err != nil {
			return builtValue{}, err
		}
	}
	fb.current = merge
	return merged, nil
}
