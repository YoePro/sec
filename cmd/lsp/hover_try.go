package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// tryExpressionHover presents only the error-flow facts resolved by Sema for
// the try expression whose keyword or protected root operand is hovered. It
// does not reconstruct Result, Option, propagation, or handler semantics.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
//   - rules/errors/errorhandling.md — §§ 8–9, 12, 15–16
func tryExpressionHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		tryExpression, ok := expression.(*ast.TryExpression)
		if !ok {
			continue
		}

		rng := tokenRange(text, tryExpression.Token)
		if !sameSourceToken(tryExpression.Token, hovered) {
			operandToken, matches := tryRootOperandToken(tryExpression.Expression, hovered)
			if !matches {
				continue
			}
			rng = tokenRange(text, operandToken)
		}

		resolved, ok := analyzer.ResolvedTryOf(tryExpression)
		if !ok {
			return hoverResult{}, false
		}
		carrier, ok := analyzer.ResolvedTypeOf(tryExpression.Expression)
		if !ok {
			return hoverResult{}, false
		}
		return hoverResult{
			Contents: markupContent{Kind: "markdown", Value: tryExpressionHoverContents(analyzer, tryExpression, carrier, resolved)},
			Range:    rng,
		}, true
	}
	return hoverResult{}, false
}

// tryRootOperandToken identifies the source token representing the protected
// operation at the root of a try operand. Nested argument tokens deliberately
// retain their own ordinary hover.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover", try and protected-operand hover
func tryRootOperandToken(expression ast.Expression, hovered lexer.Token) (lexer.Token, bool) {
	var candidates []lexer.Token
	switch expression := expression.(type) {
	case *ast.Identifier:
		candidates = append(candidates, expression.Token)
	case *ast.CallExpression:
		if expression.Function != nil {
			candidates = append(candidates, expression.Function.Token)
		}
		if member, ok := expression.Callee.(*ast.MemberExpression); ok && member.Property != nil {
			candidates = append(candidates, member.Property.Token)
		}
	case *ast.InfixExpression:
		candidates = append(candidates, expression.Token)
	case *ast.IndexExpression:
		candidates = append(candidates, expression.Token)
	case *ast.ConversionExpression:
		candidates = append(candidates, expression.Token)
	}
	for _, candidate := range candidates {
		if sameSourceToken(candidate, hovered) {
			return candidate, true
		}
	}
	return lexer.Token{}, false
}

// tryExpressionHoverContents renders the immutable Sema decision. The
// distinction between propagation and local handling comes from ResolvedTry;
// the LSP does not infer it from source syntax or carrier spelling.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
//   - rules/errors/errorhandling.md — §§ 12, 15–16 and § 37.10
func tryExpressionHoverContents(analyzer *sema.Analyzer, expression *ast.TryExpression, carrier sema.Type, resolved sema.ResolvedTry) string {
	lines := []string{"### `try`"}
	carrierLabel := "Protected operation type"
	if carrier.Kind == sema.ResultType || carrier.Kind == sema.UnionType && carrier.Name == "Option" {
		carrierLabel = "Protected carrier"
	}
	lines = append(lines,
		carrierLabel+": `"+lspTypeName(carrier)+"`",
		"Success value: `"+lspTypeName(resolved.SuccessType)+"`",
		"Try expression type: `"+lspTypeName(resolved.SuccessType)+"`",
	)

	switch resolved.Kind {
	case sema.ResolvedTryHandledResult, sema.ResolvedTryHandledArithmetic, sema.ResolvedTryHandledBounds:
		lines = append(lines,
			"Failure handling: `local try handlers`",
			"Error channel: `"+lspTypeName(resolved.ErrorType)+"`",
			"Err consumed by: `local handler`",
		)
		if plan, ok := analyzer.ResolvedTryPlanOf(expression); ok {
			coverage := "partial"
			if plan.Exhaustive {
				coverage = "exhaustive"
			}
			lines = append(lines,
				fmt.Sprintf("Handler coverage: `%s`", coverage),
				fmt.Sprintf("Resolved handlers: `%d`", len(plan.Handlers)),
			)
		}
	default:
		lines = append(lines,
			"Failure handling: `propagated`",
			"Propagated error: `"+lspTypeName(resolved.ErrorType)+"`",
			"Propagation target: `"+lspTypeName(resolved.EnclosingResultType)+"`",
			"Err consumed by: `enclosing function return`",
		)
	}

	return strings.Join(lines, "\n\n")
}
