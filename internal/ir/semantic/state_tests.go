package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildStateTest lowers `value is Variant` and `value is not Variant` in any
// value position. The subject is evaluated once, its active variant is tested,
// and the two edges carry the constant results to a merge. A Result tests Err;
// both of its edges unwrap the projected payload first, as the Result guard
// verifier requires. Only trivially copyable subjects are read this way; the
// empty state needs a runtime initialization flag Semantic IR does not have.
//
// Rules:
//   - rules/declarations/unions.md — state tests; MD-006 (`is not`)
//   - rules/control-flow/flowcontrol_if.md — state-test conditions
func (fb *functionBuilder) buildStateTest(expr *ast.StateTestExpression) (builtValue, error) {
	fact, ok := fb.owner.analyzer.ResolvedStateTestOf(expr)
	if !ok {
		return builtValue{}, fb.unsupported("unresolved state test", expr.Token)
	}
	if fact.Empty || fact.MaybeEmpty {
		return builtValue{}, fb.unsupported("empty-state test (needs a runtime initialization flag)", expr.Token)
	}
	if sema.CopyClassificationOf(fact.SubjectType) != sema.CopyTrivial {
		return builtValue{}, fb.unsupported("state test of a subject that needs ownership", expr.Token)
	}
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	subject, err := fb.buildExpr(expr.Subject, 0)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	typ, _ := fb.owner.module.Types.Lookup(subject.typ)
	matched, other := fb.newBlock(), fb.newBlock()
	switch typ.Kind {
	case TypeResult:
		isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{subject.id}, Location: loc}, boolType)
		errBlock, okBlock := matched, other
		if fact.Variant == "Ok" {
			errBlock, okBlock = other, matched
		}
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errBlock.ID}, {Block: okBlock.ID}}, Location: loc})
		fb.current = errBlock
		fb.result(Operation{Kind: OpResultUnwrapErr, Operands: []ValueID{subject.id}, Location: loc}, typ.Error)
		if !fb.owner.isVoidType(typ.Success) {
			fb.current = okBlock
			fb.result(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{subject.id}, Location: loc}, typ.Success)
		}
	case TypeUnion:
		definition, found := fb.owner.unionDefinition(subject.typ)
		if !found {
			return builtValue{}, fb.unsupported("state test without a union definition", expr.Token)
		}
		index, known := UnionVariantIndex(0), false
		for _, variant := range definition.Variants {
			if variant.Name == fact.Variant {
				index, known = variant.Index, true
			}
		}
		if !known {
			return builtValue{}, fb.unsupported("state test of an unknown variant "+fact.Variant, expr.Token)
		}
		isVariant := fb.result(Operation{Kind: OpUnionIsVariant, Operands: []ValueID{subject.id}, UnionVariant: index, Location: loc}, boolType)
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isVariant.id}, Successors: []BranchTarget{{Block: matched.ID}, {Block: other.ID}}, Location: loc})
	default:
		return builtValue{}, fb.unsupported("state test of a "+string(typ.Kind)+" subject", expr.Token)
	}
	merge := fb.newBlock()
	result := fb.newValue(boolType, OwnershipImmediate, loc)
	merge.Parameters = []Value{result}
	for _, edge := range []struct {
		block *Block
		value bool
	}{{matched, !fact.Negated}, {other, fact.Negated}} {
		fb.current = edge.block
		value := edge.value
		constant := fb.result(Operation{Kind: OpConstBool, Bool: &value, Location: loc}, boolType)
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{constant.id}}}, Location: loc})
	}
	fb.current = merge
	return builtValue{id: result.ID, typ: boolType}, nil
}
