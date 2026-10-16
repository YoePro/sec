package sema

import "sec/internal/ast"

// checkDynamicArrayMutationBorrow prevents a compiler-known structural owner
// operation from invalidating live direct references or slices. It consumes
// the resolved member contract and the existing canonical Place/liveness model;
// ordinary element writes retain their narrower Place authority. Arguments have
// already been evaluated, so their own diagnostics and temporary uses survive.
// Rules: rules/collections/collections.md — §§6.5,6.7–6.9,19;
// rules/memory/borrowing.md — §§8.1,12(6),14.5.
func (a *Analyzer) checkDynamicArrayMutationBorrow(call *ast.CallExpression) bool {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return false
	}
	contract, known := a.CompilerKnownMemberAt(member.Property.Token.File, member.Property.Token.Line, member.Property.Token.Column)
	if !known || !contract.StructuralMutation {
		return false
	}
	receiver, typed := a.expressionTypes[member.Object]
	owner := dereferenceType(receiver)
	if !typed || owner.Kind != ArrayType || arrayShapeOf(owner) != ArrayShapeDynamic {
		return false
	}
	place, resolved := a.resolvePlace(member.Object)
	if !resolved {
		return false
	}
	if receiver.Kind == ReferenceType && receiver.Element != nil {
		place = a.canonicalDereferencePlace(member.Object, place, receiver)
	}
	return a.checkBorrowedMutationPlaceForAction(place, member.Property.Token, contract.Name)
}
