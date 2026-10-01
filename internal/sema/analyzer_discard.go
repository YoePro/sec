package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// analyzeDiscardStatement validates and commits explicit terminal ownership.
//
// Rules:
//   - rules/control-flow/discard.md — §§3, 5, 7, 9, 26, 35
//   - rules/memory/ownership.md — §23
func (a *Analyzer) analyzeDiscardStatement(stmt *ast.DiscardStatement) {
	if stmt.Value == nil {
		a.addErrorAtToken(stmt.Token, "discard requires expression")
		return
	}
	if ident, ok := stmt.Value.(*ast.Identifier); ok && ident.Value == "_" {
		a.addErrorAtToken(ident.Token, "discard requires named value")
		return
	}
	if _, ok := stmt.Value.(*ast.SpawnExpression); ok {
		typ, _ := a.inferExpression(stmt.Value)
		if typ.Kind != InvalidType {
			a.addErrorAtToken(stmt.Token, "cannot discard spawn result because successful creation would abandon %s", typeDisplayName(typ))
		}
		return
	}

	// rules/control-flow/discard.md section 5 and destruction.md section 12:
	// discard is an ownership-state convergence operation, not an ordinary
	// read. Resolve an already unavailable Place without calling
	// inferExpression, which would incorrectly report use-after-move/discard.
	// Legality and outstanding borrow/defer obligations are still checked
	// before accepting the no-op or conditionally required destruction.
	if place, ok := a.resolvePlace(stmt.Value); ok {
		if a.rejectOrdinaryMethodWholeSelfConsumption(place, expressionToken(stmt.Value)) {
			return
		}
		if _, _, partial, unavailable := a.unavailablePlace(place); unavailable && !partial {
			if !a.validateExplicitDiscardType(place.Type, expressionToken(stmt.Value)) {
				return
			}
			if a.checkBorrowedMovePlaceForAction(place, expressionToken(stmt.Value), "discard") {
				return
			}
			// Preserve the resolved type and definition facts normally recorded by
			// expression inference so CLI and LSP consumers observe the same valid
			// operand even though availability checking is intentionally skipped.
			a.expressionTypes[stmt.Value] = place.Type
			if ident, isIdentifier := stmt.Value.(*ast.Identifier); isIdentifier {
				if symbol, exists := a.symbols[ident.Value]; exists {
					a.bindDefinition(ident.Token, symbol.Token)
				}
			}
			return
		}
	}

	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}
	if !a.validateExplicitDiscardType(valueType, expressionToken(stmt.Value)) {
		return
	}
	// A directly owned struct field is an independently tracked Place. Discard
	// consumes that field even when its type is copyable.
	//
	// Rules:
	//   - rules/control-flow/discard.md — §§3, 7, 26, 35
	//   - rules/memory/ownership.md — §23
	if member, ok := stmt.Value.(*ast.MemberExpression); ok {
		if place, resolved := a.resolvePlace(member); resolved && place.PartialMoveSafe &&
			len(place.Projections) > 0 && place.Projections[len(place.Projections)-1].Kind == PlaceField {
			if a.checkBorrowedMovePlaceForAction(place, expressionToken(stmt.Value), "discard") {
				return
			}
			a.markPlaceUnavailable(place, expressionToken(stmt.Value), "discarded")
			return
		}
	}

	ident, ok := stmt.Value.(*ast.Identifier)
	if !ok {
		// rules/control-flow/discard.md, aggregate temporary discard;
		// correction19.md requires construction-time moves to be committed before
		// the resulting temporary is destroyed. Calls retain their parameter-mode
		// ownership handling because markMoveSource deliberately does not descend
		// into call expressions.
		a.markMoveSource(stmt.Value)
		return
	}
	symbol, exists := a.symbols[ident.Value]
	if !exists {
		return
	}
	place, placeOK := a.rootPlace(ident.Value)
	if placeOK && a.checkBorrowedMovePlaceForAction(place, ident.Token, "discard") {
		return
	}
	a.moved[ident.Value] = ident.Token
	a.moveReasons[ident.Value] = "discarded"
	delete(a.constInts, ident.Value)
	a.endBorrowsHeldBy(ident.Value)
	if symbol.Type.Kind == ReferenceType {
		delete(a.localRefContainers, ident.Value)
	}
}

// validateExplicitDiscardType implements the type-level part of explicit
// discard from rules/control-flow/discard.md section 5 and
// rules/memory/destruction.md section 12. It keeps discardability independent
// from current availability: an unavailable lifecycle handle does not become
// discardable merely because consuming it would perform no second destruction.
func (a *Analyzer) validateExplicitDiscardType(valueType Type, token lexer.Token) bool {
	if isTaskType(valueType) || isThreadType(valueType) {
		a.addErrorAtToken(token, "cannot discard unresolved %s; await, join or detach it explicitly", typeDisplayName(valueType))
		return false
	}
	if isDiscardableType(valueType) {
		return true
	}
	a.addErrorAtTokenWithMetadata(
		token,
		diagnostics.NonDiscardableValue,
		"handle the value and resolve every contained task or thread lifecycle",
		"cannot discard %s because it may contain an unresolved lifecycle handle",
		typeDisplayName(valueType),
	)
	return false
}
