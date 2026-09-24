package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// isSelfMemberProjection recognizes a projected Place rooted at implicit self
// without requiring body-analysis symbols to have been installed yet.
//
// Rules:
//   - rules/memory/ownership.md — §8 "Field mutability and receiver authority"
//   - rules/memory/copy_move.md — §18 "Methods and self"
func isSelfMemberProjection(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.MemberExpression:
		return expressionUsesSelf(expression.Object)
	case *ast.IndexExpression:
		return isSelfMemberProjection(expression.Left)
	default:
		return false
	}
}

// rejectOrdinaryMethodWholeSelfConsumption prevents a normal instance method
// from ending ownership of its complete receiver. Projected members remain
// governed by partial-move, receiver-authority, and borrow validation.
//
// Rules:
//   - rules/memory/ownership.md — §9 "Methods and self"
//   - rules/memory/copy_move.md — §18 "Methods and self"
//   - rules/declarations/functions.md — §22 "Instance methods"
func (a *Analyzer) rejectOrdinaryMethodWholeSelfConsumption(place Place, token lexer.Token) bool {
	if a.currentFunctionMetadata.ImplTarget == "" || a.currentFunctionMetadata.Static || a.currentFunctionMetadata.Initializer ||
		place.Root != "self" || len(place.Projections) != 0 {
		return false
	}
	a.addErrorAtToken(token, "ordinary method cannot consume complete self; consume an owned member or use destruction")
	return true
}

// validateNamedOwnershipSource enforces explicit ownership syntax and source
// Place legality before an initialization, assignment, call, payload, or
// terminal return can commit a transfer.
//
// Rules:
//   - rules/memory/ownership.md — §§10–14 "Explicit move syntax" and call transfer
//   - rules/memory/copy_move.md — §§5–9 copy and move boundaries
//   - rules/declarations/functions.md — §22 "Instance methods"
func (a *Analyzer) validateNamedOwnershipSource(mode ast.OwnershipMode, value ast.Expression, token lexer.Token, declaration bool, inferredDeclaration bool) bool {
	place, ok := a.resolvePlace(value)
	if !ok {
		if mode == ast.OwnershipMove {
			a.addErrorAtToken(token, "explicit move requires a reusable source place")
			return false
		}
		return true
	}
	if mode == ast.OwnershipMove && a.rejectOrdinaryMethodWholeSelfConsumption(place, token) {
		return false
	}
	if mode == ast.OwnershipMove && a.rejectPhysicalStorageMove(place, token) {
		return false
	}
	if a.variadicPackElementExpression(value) && (mode == ast.OwnershipMove || requiresOwnershipTransfer(place.Type)) {
		// rules/declarations/functions.md section 34: no direct or partial
		// move may extract an element from the invocation-lifetime pack.
		a.addErrorAtToken(expressionToken(value), "cannot move element out of variadic parameter pack")
		return false
	}
	if _, _, _, unavailable := a.unavailablePlace(place); unavailable {
		// inferExpression has already emitted the primary use-after-move error;
		// do not apply ownership again and overwrite the original move site.
		return false
	}
	if mode == ast.OwnershipMove {
		if !place.Addressable {
			a.addErrorAtToken(token, "explicit move requires an addressable source place")
			return false
		}
		if _, isIndex := value.(*ast.IndexExpression); isIndex {
			a.addErrorAtToken(token, "explicit indexed extraction is not implemented; move the containing value")
			return false
		}
		if len(place.Projections) > 0 && !place.PartialMoveSafe {
			a.addErrorAtToken(token, "partial move requires independently tracked local struct storage")
			return false
		}
		return !a.checkBorrowedMovePlace(place, expressionToken(value))
	}
	classification := CopyClassificationOf(place.Type)
	if classification == CopyTrivial || classification == CopySemantic {
		return true
	}
	moveSyntax := "<-"
	if declaration && inferredDeclaration {
		moveSyntax = ":<-"
	}
	help := "use `destination <- source` to transfer ownership"
	if declaration {
		help = "use `let destination :<- source` to transfer ownership"
	}
	switch classification {
	case CopyNonCopyable:
		a.addErrorAtTokenWithMetadata(
			expressionToken(value),
			diagnostics.ImplicitMoveDisallowed,
			help,
			"%s value %s cannot be copied because %s; use explicit move syntax %s",
			typeDisplayName(place.Type),
			place.String(),
			nonCopyableCause(place.Type),
			moveSyntax,
		)
		return false
	case CopyConditional:
		a.addErrorAtTokenWithMetadata(
			expressionToken(value),
			diagnostics.ImplicitMoveDisallowed,
			help,
			"cannot copy value %s because generic copyability has not been proven; use explicit move syntax %s",
			place.String(),
			moveSyntax,
		)
		return false
	}
	a.addErrorAtTokenWithMetadata(
		expressionToken(value),
		diagnostics.ImplicitMoveDisallowed,
		help,
		"cannot copy move-only value %s; use explicit move syntax %s",
		place.String(),
		moveSyntax,
	)
	return false
}

// rejectPhysicalStorageMove keeps ownership transfer distinct from an
// observable read of addressed volatile/MMIO storage. Reading such a Place
// produces an ordinary local snapshot, but the physical Place itself is not a
// reusable Sec-owned value that can be consumed with :<- or <-.
//
// Rules:
//   - rules/memory/copy_move.md — §19.1 "Volatile is storage semantics" and §19.2 "Volatile storage is not a movable owner"
//   - rules/memory/ownership.md — §30 "Hardware, fixed-address storage, and FFI"
//   - rules/platform/volatile.md — physical storage access and diagnostics
func (a *Analyzer) rejectPhysicalStorageMove(place Place, token lexer.Token) bool {
	symbol, ok := a.symbols[place.Root]
	if !ok || !symbol.Volatile && !symbol.Addressed {
		return false
	}
	a.addErrorAtToken(
		token,
		"cannot move ownership out of volatile/MMIO storage %s; read it into a local snapshot first",
		place.String(),
	)
	return true
}

// markExplicitMoveSource commits one already validated explicit Place move and
// ends borrows held by a wholly moved non-reference root.
//
// Rules:
//   - rules/memory/ownership.md — §10 "Explicit move syntax"
//   - rules/memory/borrowing.md — move/borrow exclusion
func (a *Analyzer) markExplicitMoveSource(expr ast.Expression) bool {
	place, ok := a.resolvePlace(expr)
	if !ok || a.rejectOrdinaryMethodWholeSelfConsumption(place, expressionToken(expr)) {
		return false
	}
	if a.checkBorrowedMovePlace(place, expressionToken(expr)) {
		return false
	}
	a.markPlaceUnavailable(place, expressionToken(expr), "moved")
	if len(place.Projections) == 0 && place.Type.Kind != ReferenceType {
		a.endBorrowsHeldBy(place.Root)
	}
	return true
}

// inferCallArgumentExpression admits the ownership marker only at the direct
// argument boundary. Transfer is validated after overload selection and is not
// committed until every argument required for outer-call entry is ready.
//
// Rules:
//   - rules/memory/ownership.md — §§10.4 and 13–14
//   - rules/memory/copy_move.md — §§7–8
func (a *Analyzer) inferCallArgumentExpression(arg ast.Expression) (Type, expressionValue) {
	move, explicit := explicitMoveArgument(arg)
	if !explicit {
		return a.inferExpression(arg)
	}
	typ, value := a.inferExpression(move.Right)
	if typ.Kind != InvalidType {
		a.expressionTypes[arg] = typ
	}
	return typ, value
}

// explicitMoveArgument recognizes the direct call-boundary `<-source` form.
//
// Rules:
//   - rules/memory/copy_move.md — §8.2 "Consuming parameters"
func explicitMoveArgument(expr ast.Expression) (*ast.PrefixExpression, bool) {
	move, ok := expr.(*ast.PrefixExpression)
	return move, ok && move.Operator == "<-" && move.Right != nil
}

// explicitMoveSource unwraps a validated direct call-boundary move marker.
//
// Rules:
//   - rules/memory/copy_move.md — §8.2 "Consuming parameters"
func explicitMoveSource(expr ast.Expression) ast.Expression {
	if move, ok := explicitMoveArgument(expr); ok {
		return move.Right
	}
	return expr
}

// validateCallArgumentOwnership requires visible transfer from every reusable
// Place passed to a consuming parameter, including copyable Places, while
// preserving marker-free forwarding of fresh temporaries.
//
// Rules:
//   - rules/memory/copy_move.md — §§7 and 8.2
//   - rules/declarations/functions.md — §9 "Explicit consuming parameter"
func (a *Analyzer) validateCallArgumentOwnership(function Function, sourceArgs []ast.Expression, preparedSpreadValues []bool) bool {
	valid := true
	for sourceIndex, arg := range sourceArgs {
		param, ok := functionParameterForArgument(function, sourceIndex)
		if !ok {
			return false
		}
		preparedSpread := sourceIndex < len(preparedSpreadValues) && preparedSpreadValues[sourceIndex]
		move, explicit := explicitMoveArgument(arg)
		source := explicitMoveSource(arg)
		borrowParameter := param.Ref || param.MutableRef || param.Type.Kind == ReferenceType
		if explicit {
			if preparedSpread || borrowParameter {
				a.addErrorAtToken(move.Token, "explicit <- argument requires an owning value parameter")
				valid = false
				continue
			}
			if !a.validateNamedOwnershipSource(ast.OwnershipMove, source, move.Token, false, false) {
				valid = false
			}
			continue
		}
		if preparedSpread || borrowParameter {
			continue
		}
		place, reusable := a.resolvePlace(source)
		if !reusable {
			continue
		}
		needsTransfer := param.Consuming || requiresOwnershipTransfer(place.Type)
		if !needsTransfer {
			continue
		}
		a.addErrorAtTokenWithMetadata(
			expressionToken(source),
			diagnostics.ImplicitMoveDisallowed,
			fmt.Sprintf("write `<-%s` to make the ownership transfer explicit", place.String()),
			"reusable source %s passed to %s must use explicit <- ownership transfer",
			place.String(),
			function.Name,
		)
		valid = false
	}
	return valid
}

// consumeMethodReceiver records ownership termination for a consuming method
// receiver after the call has been selected and validated.
//
// Rules:
//   - rules/memory/ownership.md — §9 "Methods and self"
//   - rules/memory/copy_move.md — §18 "Methods and self"
func (a *Analyzer) consumeMethodReceiver(expression ast.Expression) {
	identifier, ok := expression.(*ast.Identifier)
	if !ok {
		a.markMoveSource(expression)
		return
	}
	if a.checkBorrowedMove(identifier.Value, identifier.Token) {
		return
	}
	a.moved[identifier.Value] = identifier.Token
	a.moveReasons[identifier.Value] = "consumed by method call"
	a.endBorrowsHeldBy(identifier.Value)
}

// markMovedCallArguments commits validated argument transfers only after every
// outer-call argument is ready. Borrowed and prepared spread values do not
// consume their source Places.
//
// Rules:
//   - rules/memory/copy_move.md — §§8.2–8.3
//   - rules/declarations/functions.md — §§9 and 13
//   - rules/declarations/spread.md — consuming destinations
func (a *Analyzer) markMovedCallArguments(function Function, sourceArgs []ast.Expression, preparedSpreadValues []bool, isMethodCall bool) {
	for sourceIndex, arg := range sourceArgs {
		param, parameterOK := functionParameterForArgument(function, sourceIndex)
		if !parameterOK {
			return
		}
		preparedSpread := sourceIndex < len(preparedSpreadValues) && preparedSpreadValues[sourceIndex]
		if param.Ref || param.MutableRef || param.Type.Kind == ReferenceType {
			continue
		}
		if preparedSpread {
			continue
		}
		source := explicitMoveSource(arg)
		_, explicit := explicitMoveArgument(arg)
		if param.Consuming || explicit {
			if a.markExplicitMoveSource(source) {
				if ident, ok := source.(*ast.Identifier); ok {
					a.moveReasons[ident.Value] = "consumed by call"
					a.endBorrowsHeldBy(ident.Value)
				}
			}
			continue
		}
		if a.markMoveSource(source) {
			if ident, ok := source.(*ast.Identifier); ok {
				a.endBorrowsHeldBy(ident.Value)
			}
		}
	}
}

// validateLambdaCaptureOwnership enforces the revision-2 distinction between
// plain copy capture and explicit consuming capture before the lambda receives
// its distinct environment binding. A successful move immediately updates the
// outer Place; inferLambdaExpression analyzes the environment binding with an
// isolated availability map.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§15–17 "Capture forms"
//   - rules/memory/copy_move.md — §11 "Closure captures"
//   - rules/memory/ownership.md — §16 "Closure captures"
func (a *Analyzer) validateLambdaCaptureOwnership(expr *ast.LambdaExpression) {
	seen := map[string]bool{}
	for _, capture := range expr.Captures {
		if capture.Name == nil || seen[capture.Name.Value] {
			continue
		}
		seen[capture.Name.Value] = true

		symbol, exists := a.symbols[capture.Name.Value]
		if !exists || !symbol.Local || !a.assigned[capture.Name.Value] {
			// Capture resolution owns the primary undefined, non-local, and
			// unassigned diagnostics.
			continue
		}
		place, ok := a.resolvePlace(capture.Name)
		if !ok || a.checkPlaceAvailableForRead(place, capture.Name.Token) {
			continue
		}

		if capture.Mode == ast.LambdaCaptureMove {
			if a.validateNamedOwnershipSource(ast.OwnershipMove, capture.Name, capture.Token, false, false) {
				a.markExplicitMoveSource(capture.Name)
			}
			continue
		}

		classification := CopyClassificationOf(symbol.Type)
		if classification == CopyTrivial || classification == CopySemantic {
			continue
		}
		help := fmt.Sprintf("use `capture(<-%s)` to transfer ownership or capture a reference", capture.Name.Value)
		switch classification {
		case CopyNonCopyable:
			a.addErrorAtTokenWithMetadata(
				capture.Name.Token,
				diagnostics.ImplicitMoveDisallowed,
				help,
				"cannot copy-capture %s because %s; plain capture never consumes its source",
				capture.Name.Value,
				nonCopyableCause(symbol.Type),
			)
		case CopyConditional:
			a.addErrorAtTokenWithMetadata(
				capture.Name.Token,
				diagnostics.ImplicitMoveDisallowed,
				help,
				"cannot copy-capture %s because generic copyability has not been proven; plain capture never consumes its source",
				capture.Name.Value,
			)
		default:
			a.addErrorAtTokenWithMetadata(
				capture.Name.Token,
				diagnostics.ImplicitMoveDisallowed,
				help,
				"cannot copy-capture move-only value %s; plain capture never consumes its source",
				capture.Name.Value,
			)
		}
	}
}
