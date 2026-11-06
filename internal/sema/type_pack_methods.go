package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// typePackOwnerSupported admits free functions and non-interface impl owners; a
// generic owner's arguments compose with the method's own parameters. An
// interface requirement has no static expansion, because dispatch is not static.
// Rules: rules/declarations/functions.md §§21–24,38A(1–4);
// rules/compiler/monomorphization.md §§8.2,9.3.
func (a *Analyzer) typePackOwnerSupported(owner string) bool {
	if owner == "" {
		return true
	}
	target, known := a.types[owner]
	return known && target.Kind != InterfaceType
}

// implicitTypePackReceiver supplies the enclosing ordinary method's existing
// self Place for an unqualified pack-method call. Type-qualified calls do not
// manufacture an instance, and static callers supply no implicit receiver.
// Rules: rules/declarations/functions.md §§22–24; rules/declarations/impl.md §8.
func (a *Analyzer) implicitTypePackReceiver(function Function, call *ast.CallExpression) (methodReceiverInfo, bool) {
	if _, named := call.Callee.(*ast.Identifier); !named || !functionUsesReceiver(a.currentFunctionMetadata) || function.ImplTarget != a.currentFunctionMetadata.ImplTarget {
		return methodReceiverInfo{}, false
	}
	self, known := a.symbols.lookup("self")
	if !known {
		return methodReceiverInfo{}, false
	}
	return methodReceiverInfo{Type: self.Type, Symbol: &self, Expr: &ast.Identifier{Token: call.Token, Value: "self"}}, true
}

// validateTypePackReceiverArguments preserves receiver authority alongside pack
// argument loans: shared/exclusive access must not overlap while the callee
// executes, and an owning argument cannot end storage borrowed by its receiver.
// Copyable by-value arguments are separate prepared values, not competing loans.
// Rules: rules/declarations/functions.md §§22,38A(3)(2–4),(6);
// rules/memory/borrowing.md §§7(5),15.2–15.4.
func (a *Analyzer) validateTypePackReceiverArguments(function Function, receiver methodReceiverInfo, args []ast.Expression) bool {
	if function.GenericTypePack == "" || !functionUsesReceiver(function) {
		return true
	}
	receiverPlace, known := a.callArgumentBorrowPlace(receiver.Expr)
	if !known {
		return true
	}
	for i, argument := range args {
		parameter, known := functionParameterForArgument(function, i)
		if !known {
			return false
		}
		mutable, borrowed := callParameterBorrowMode(parameter)
		_, moved := explicitMoveArgument(argument)
		if !borrowed && !moved {
			continue
		}
		if borrowed && !mutable && !function.ReceiverMutable {
			continue
		}
		var place Place
		if borrowed {
			place, known = a.callArgumentBorrowPlace(argument)
		} else {
			place, known = a.resolvePlace(explicitMoveSource(argument))
		}
		if !known || !placesMayOverlap(receiverPlace, place) {
			continue
		}
		id := diagnostics.BorrowMutableConflict
		if moved {
			id = diagnostics.BorrowOwnershipEndConflict
		}
		a.addErrorAtTokenWithMetadata(expressionToken(argument), id,
			"Use separate storage for this argument, or separate the receiver operation from the conflicting borrow or move.",
			"argument %d conflicts with the receiver loan of method %s", i+1, function.Name)
		return false
	}
	return true
}
