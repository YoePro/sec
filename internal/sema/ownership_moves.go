package sema

import "sec/internal/ast"

// markMoveSource commits source ownership for an accepted owning transfer,
// including aggregate/payload forwarding, and records cleanup cancellation.
// Rules: rules/memory/copy_move.md — §§9–10;
// rules/memory/destruction.md — §6.1(3), §9.
func (a *Analyzer) markMoveSource(expr ast.Expression) bool {
	if moved, known := a.visitUnionOwnershipOperands(expr, a.markMoveSource); known {
		return moved
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols.lookup(expr.Value)
		if !ok || !requiresOwnershipTransfer(symbol.Type) {
			return false
		}
		if a.checkBorrowedMove(expr.Value, expr.Token) || a.rejectBorrowedArenaMove(expr.Value, expr.Token) {
			return false
		}
		if place, ok := a.rootPlace(expr.Value); ok {
			a.recordOwnedCleanupCancellation(place, expr.Token, "moved")
		}
		a.moved[expr.Value] = expr.Token
		a.availabilityStates[expr.Value] = unavailableState("moved")
		a.recordMoveSource(expr)
		return true
	case *ast.MemberExpression:
		place, ok := a.resolvePlace(expr)
		if !ok || !requiresOwnershipTransfer(place.Type) || a.rejectCustomFreePartialMove(place, expr.Property.Token) || a.checkBorrowedMovePlace(place, expr.Property.Token) {
			return false
		}
		a.markPlaceUnavailable(place, expr.Property.Token, "moved")
		a.recordMoveSource(expr)
		return true
	case *ast.StructLiteral:
		moved := false
		for _, field := range expr.Fields {
			if field != nil && a.markMoveSource(field.Value) {
				moved = true
			}
		}
		return moved
	case *ast.ArrayLiteral:
		moved := false
		for _, element := range expr.Elements {
			if a.markMoveSource(element) {
				moved = true
			}
		}
		return moved
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.markMoveSource(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.markMoveSource(expr.Value)
		}
	case *ast.PrefixExpression:
		if expr.Operator == "<-" && expr.Right != nil {
			return a.markExplicitMoveSource(expr.Right)
		}
	case *ast.SpreadExpression:
		return a.markMoveSource(expr.Value)
	case *ast.BitExtractionExpression:
		return a.markMoveSource(expr.Value)
	case *ast.ConversionExpression:
		return a.markMoveSource(expr.Value)

	}
	return false
}

// unionConstructorPayload recognizes the resolved single-payload constructor
// boundary so ownership follows its payload rather than ordinary call inference.
// Rules: rules/declarations/unions.md — §5.2 "Single-payload variants";
// rules/memory/copy_move.md — §10.
func (a *Analyzer) unionConstructorPayload(expr *ast.CallExpression) (Type, bool) {
	plan, known := a.resolvedUnionConstructions[expr]
	if !known || plan.Kind != ResolvedUnionVariantSingle || len(plan.PayloadPreparation) != 1 {
		return Type{}, false
	}
	return plan.PayloadPreparation[0].Type, true
}

// markResourceTransfer commits return transfer of close-tracked resources and
// their owning aggregate/payload continuations without retaining old cleanup.
// Rules: rules/memory/copy_move.md — §9;
// rules/memory/destruction.md — §6.1(3), §9.3.
func (a *Analyzer) markResourceTransfer(expr ast.Expression) bool {
	if moved, known := a.visitUnionOwnershipOperands(expr, a.markResourceTransfer); known {
		return moved
	}
	switch expr := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols.lookup(expr.Value)
		if !ok || !isCloseTrackedResourceType(symbol.Type) {
			return false
		}
		if a.checkBorrowedMove(expr.Value, expr.Token) {
			return false
		}
		a.moved[expr.Value] = expr.Token
		a.availabilityStates[expr.Value] = unavailableState("returned")
		if place, ok := a.rootPlace(expr.Value); ok {
			a.recordOwnedCleanupCancellation(place, expr.Token, "returned")
		}
		a.endBorrowsHeldBy(expr.Value)
		return true
	case *ast.OkExpression:
		if expr.Value != nil {
			return a.markResourceTransfer(expr.Value)
		}
	case *ast.ErrExpression:
		if expr.Value != nil {
			return a.markResourceTransfer(expr.Value)
		}
	case *ast.StructLiteral:
		transferred := false
		for _, field := range expr.Fields {
			if field != nil && a.markResourceTransfer(field.Value) {
				transferred = true
			}
		}
		return transferred
	case *ast.ArrayLiteral:
		transferred := false
		for _, element := range expr.Elements {
			if a.markResourceTransfer(element) {
				transferred = true
			}
		}
		return transferred
	case *ast.PrefixExpression:
		if expr.Operator == "<-" {
			return a.markResourceTransfer(expr.Right)
		}

	case *ast.BitExtractionExpression:
		return a.markResourceTransfer(expr.Value)
	case *ast.ConversionExpression:
		return a.markResourceTransfer(expr.Value)
	}
	return false
}

// recordMoveSource publishes an accepted ownership transfer of a Place for
// consumers that execute the program, such as semantic CTE.
func (a *Analyzer) recordMoveSource(expr ast.Expression) {
	if a.moveSources == nil {
		a.moveSources = map[ast.Expression]bool{}
	}
	a.moveSources[expr] = true
}
