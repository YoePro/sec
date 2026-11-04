package sema

import "sec/internal/ast"

// recordReceiverMove publishes the source Place of an accepted consuming
// receiver at its commit point, separate from argument transfer plans. Fresh
// receivers have no reusable Place and therefore need no source-move fact.
// Rules: rules/errors/errorhandling.md — §§6.1,29 (Result.Ok/Err);
// rules/memory/copy_move.md — §18;
// rules/compiler/compiler_analysis.md — §55 snapshots.
func (a *Analyzer) recordReceiverMove(call *ast.CallExpression, place Place) {
	if a.summaryPass || call == nil {
		return
	}
	if a.receiverMoves == nil {
		a.receiverMoves = map[*ast.CallExpression]Place{}
	}
	a.receiverMoves[call] = cloneSlicePlanPlace(place)
}

// ResolvedReceiverMoveOf returns the detached reusable source consumed by a
// validated call. Absence is not a claim that an unresolved call is borrowing.
// Rules: rules/tooling/lsp.md — "Inlay hints" (compiler-owned ownership facts);
// rules/compiler/compiler_analysis.md — §55 immutable analysis results.
func (a *Analyzer) ResolvedReceiverMoveOf(call *ast.CallExpression) (Place, bool) {
	if a == nil || call == nil {
		return Place{}, false
	}
	place, ok := a.receiverMoves[call]
	if !ok {
		return Place{}, false
	}
	return cloneSlicePlanPlace(place), true
}
