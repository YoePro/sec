package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildCheckedEnumConversion exposes the enum's unchanged integer carrier and
// reuses the ordinary checked-integer candidate/failure routing. A surrounding
// try receives ConversionError.OutOfRange; otherwise the same edges construct
// a first-class Result. Fallibility always wins over an inconsistent proof.
// Rules: rules/declarations/enums.md — §12.1; rules/types/types.md — Exact integer conversion proof;
// rules/compiler/semantic_ir.md — §124; rules/errors/errorhandling.md — §§5,11.
func (fb *functionBuilder) buildCheckedEnumConversion(call *ast.CallExpression, conversion sema.ResolvedEnumConversion) (builtValue, error) {
	carrier, ok := fb.owner.lookupEnumUnderlying(conversion.EnumType)
	if !ok || (carrier.Kind != sema.IntType && carrier.Kind != sema.UintType) {
		return builtValue{}, fb.unsupported("checked non-integer enum representation", call.Token)
	}
	integer := sema.ResolvedIntegerConversion{SourceType: carrier, TargetType: conversion.IntegerType}
	operand := func(carrierID TypeID) (builtValue, error) {
		enumID, err := fb.owner.internType(conversion.EnumType)
		if err != nil {
			return builtValue{}, err
		}
		value, err := fb.buildExpr(call.Arguments[0], enumID)
		if err != nil {
			return builtValue{}, err
		}
		// An enum value always fits its own declared representation. This
		// extraction does not claim that the later target conversion is exact.
		proof := &ConversionProof{Basis: string(sema.IntegerConversionSourceDomain)}
		return fb.result(Operation{Kind: OpEnumToInteger, Operands: []ValueID{value.id}, ConversionProof: proof, Location: location(call.Token)}, carrierID), nil
	}
	if point, protected := fb.failurePoint(call); protected {
		return fb.buildCheckedIntegerConversionOperand(call, integer, point, operand)
	}
	typ, resolved := fb.owner.analyzer.ResolvedTypeOf(call)
	if !resolved || typ.Kind != sema.ResultType || len(typ.TypeArgs) != 2 {
		return builtValue{}, fb.unsupported("checked enum conversion Result facts", call.Token)
	}
	resultType, err := fb.owner.internType(typ)
	if err != nil {
		return builtValue{}, err
	}
	previous := fb.failures
	router := &failureRouter{points: map[ast.Expression]sema.TryFailurePoint{}, routed: map[ast.Expression]bool{}}
	fb.failures = router
	defer func() { fb.failures = previous }()
	success, err := fb.buildCheckedIntegerConversionOperand(call, integer, sema.TryFailurePoint{ErrorType: typ.TypeArgs[1]}, operand)
	if err != nil {
		return builtValue{}, err
	}
	merge := fb.newBlock()
	parameter := fb.newValue(resultType, OwnershipImmediate, location(call.Token))
	merge.Parameters = []Value{parameter}
	value := fb.result(Operation{Kind: OpResultOk, Operands: []ValueID{success.id}, Location: location(call.Token)}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{value.id}}}, Location: location(call.Token)})
	for _, target := range router.targets {
		fb.current = target.block
		value := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{target.value}, Location: location(call.Token)}, resultType)
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{value.id}}}, Location: location(call.Token)})
	}
	fb.current = merge
	return builtValue{id: parameter.ID, typ: resultType}, nil
}
