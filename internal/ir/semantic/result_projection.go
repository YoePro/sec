package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildResultProjection lowers the consuming Result.Ok()/Err(). The receiver is
// consumed as a try operand is; on the retained side its payload moves or copies
// into Some, and on the other side the inactive payload ends by its destruction
// plan before None is built (errorhandling.md § 6.1; destruction.md § 16.4).
// Rules: rules/errors/errorhandling.md — §§ 6.1, 29; rules/memory/destruction.md — § 16.4.
func (fb *functionBuilder) buildResultProjection(call *ast.CallExpression, member *ast.MemberExpression, ok bool) (builtValue, error) {
	token := member.Property.Token
	loc := location(token)
	resultSema, typed := fb.owner.analyzer.ResolvedTypeOf(member.Object)
	optionSema, optionTyped := fb.owner.analyzer.TypeOf(call)
	if !typed || !optionTyped || resultSema.Kind != sema.ResultType || len(resultSema.TypeArgs) != 2 {
		return builtValue{}, fb.unsupported("Result projection without a resolved Result and Option", token)
	}
	retained, dropped := resultSema.TypeArgs[0], resultSema.TypeArgs[1]
	if !ok {
		retained, dropped = dropped, retained
	}
	if retained.Kind == sema.VoidType {
		return builtValue{}, fb.unsupported("Result projection of a void payload", token)
	}
	optionType, err := fb.owner.internType(optionSema)
	if err != nil {
		return builtValue{}, err
	}
	some, none, found := fb.optionVariants(optionType)
	if !found {
		return builtValue{}, fb.unsupported("Result projection without an Option definition", token)
	}
	result, err := fb.buildTryOperand(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	resultType, _ := fb.owner.module.Types.Lookup(result.typ)
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{result.id}, Location: loc}, boolType)
	errBlock, okBlock, merge := fb.newBlock(), fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errBlock.ID}, {Block: okBlock.ID}}, Location: loc})
	parameter := fb.newValue(optionType, OwnershipOwned, loc)
	merge.Parameters = []Value{parameter}

	retainedBlock, droppedBlock := okBlock, errBlock
	retainedKind, droppedKind := OpResultUnwrapOk, OpResultUnwrapErr
	retainedID, droppedID := resultType.Success, resultType.Error
	if !ok {
		retainedBlock, droppedBlock = errBlock, okBlock
		retainedKind, droppedKind = OpResultUnwrapErr, OpResultUnwrapOk
		retainedID, droppedID = resultType.Error, resultType.Success
	}

	fb.current = retainedBlock
	payload := fb.result(Operation{Kind: retainedKind, Operands: []ValueID{result.id}, Location: loc}, retainedID)
	action := UnionPayloadCopyTrivial
	if !fb.owner.concreteTriviallyCopyable(retained) {
		action = UnionPayloadConstructDirect
	}
	someValue := fb.result(Operation{Kind: OpUnionConstruct, Operands: []ValueID{payload.id}, UnionVariant: some, PayloadActions: []UnionPayloadAction{action}, Location: loc}, optionType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{someValue.id}}}, Location: loc})

	fb.current = droppedBlock
	if dropped.Kind != sema.VoidType {
		inactive := fb.result(Operation{Kind: droppedKind, Operands: []ValueID{result.id}, Location: loc}, droppedID)
		if !sema.TriviallyDestructible(dropped) {
			planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(dropped), dropped)
			if err != nil {
				return builtValue{}, err
			}
			fb.emit(Operation{Kind: OpOwnDestroyValue, Operands: []ValueID{inactive.id}, Ownership: &OwnershipFacts{Cause: "compiler-generated", DestructionPlan: planID}, Location: loc})
		}
	}
	noneValue := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: none, Location: loc}, optionType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{noneValue.id}}}, Location: loc})

	fb.current = merge
	return builtValue{id: parameter.ID, typ: optionType}, nil
}
