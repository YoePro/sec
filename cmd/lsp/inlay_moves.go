package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// implicitMoveHints projects accepted Sema transfers without inferring moves
// from type names or last-use heuristics. Terminal construction/return sources
// and consuming receiver Places get hints; explicit markers and fresh values
// do not. The caller applies the common range/category/deduplication filters.
// Rules: rules/tooling/lsp.md — "Inlay hints" (ownership);
// rules/memory/copy_move.md — §§9.1–9.2,10,18;
// rules/errors/errorhandling.md — §§6.1,29.
func implicitMoveHints(program *ast.Program, analyzer *sema.Analyzer, text string, inThisFile func(lexer.Token) bool) []inlayHint {
	explicit := map[ast.Expression]bool{}
	walkLSPNodes(program, func(node any) {
		switch node := node.(type) {
		case *ast.PrefixExpression:
			if node.Operator == "<-" {
				explicit[node.Right] = true
			}
		case *ast.ReturnStatement:
			if node.MoveToken.Type == lexer.MOVE_ASSIGN {
				explicit[node.Value] = true
			}
		}
	})
	var hints []inlayHint
	add := func(source ast.Expression) {
		if explicit[source] || !ownershipPlaceExpression(source) {
			return
		}
		token := leftmostExpressionToken(source)
		if !validDefinitionTokenForHint(token) || !inThisFile(token) {
			return
		}
		hints = append(hints, inlayHint{Position: tokenRange(text, token).Start, Label: "move", PaddingRight: true})
	}
	walkLSPNodes(program, func(node any) {
		if expression, ok := node.(ast.Expression); ok {
			if plan, ok := analyzer.ResolvedTransferPlanOf(expression); ok && plan.Action == sema.TransferMove &&
				(plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteConstruction) {
				add(expression)
			}
		}
		if call, ok := node.(*ast.CallExpression); ok {
			if _, consumes := analyzer.ResolvedReceiverMoveOf(call); consumes {
				if member, ok := call.Callee.(*ast.MemberExpression); ok {
					add(member.Object)
				}
			}
		}
	})
	return hints
}
