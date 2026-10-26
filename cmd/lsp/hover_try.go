package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
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
		if len(tryExpression.Handlers) > 0 {
			// A retained success type is useful for recovery, but an invalid
			// handler set has no completed compiler plan to present.
			if _, valid := analyzer.ResolvedTryPlanOf(tryExpression); !valid {
				return hoverResult{}, false
			}
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
// distinction between Result failure and Option absence, and between
// propagation and local handling, comes from ResolvedTry; the LSP does not
// infer it from source syntax or carrier spelling.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
//   - rules/errors/errorhandling.md — §§ 9, 12, 15–16 and §§ 37.4, 37.10
//   - rules/corrections/applied/lsp-errorhandling-correction-20260824.md — "Try hover"
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
	case sema.ResolvedTryOptionPropagation:
		lines = append(lines,
			"Success state: `Some("+lspTypeName(resolved.SuccessType)+")`",
			"Absence handling: `propagated`",
			"Propagated state: `None`",
			"Propagation target: `"+lspTypeName(resolved.EnclosingOptionType)+"`",
			"None consumed by: `enclosing function return`",
		)
	case sema.ResolvedTryHandledOption:
		// rules/errors/errorhandling.md — §15, §16: None is the only
		// alternate state of an Option try. Present the compiler-owned plan
		// exactly as for Result handlers so guarded recovery and residual
		// propagation remain visible instead of being collapsed into a vague
		// "local handler" label.
		lines = append(lines,
			"Success state: `Some("+lspTypeName(resolved.SuccessType)+")`",
			"Absence handling: `local None handlers`",
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
			lines = append(lines, tryHandlerHoverLines(plan.Handlers)...)
			if plan.Exhaustive {
				lines = append(lines, "None consumed by: `local handler`")
			}
			if plan.ResidualPropagates {
				lines = append(lines,
					"Unhandled None: `propagated`",
					"Propagation target: `"+lspTypeName(plan.EnclosingResultType)+"`",
				)
			}
		}
	case sema.ResolvedTryHandledResult, sema.ResolvedTryHandledArithmetic, sema.ResolvedTryHandledBounds, sema.ResolvedTryHandledFailureSet:
		lines = append(lines, "Failure handling: `local try handlers`")
		if len(resolved.Failures) > 0 {
			lines = append(lines, tryFailureSetHoverLines(resolved.Failures)...)
		} else {
			lines = append(lines, "Error channel: `"+lspTypeName(resolved.ErrorType)+"`")
		}
		lines = append(lines, "Err consumed by: `local handler`")
		if plan, ok := analyzer.ResolvedTryPlanOf(expression); ok {
			coverage := "partial"
			if plan.Exhaustive {
				coverage = "exhaustive"
			}
			lines = append(lines,
				fmt.Sprintf("Handler coverage: `%s`", coverage),
				fmt.Sprintf("Resolved handlers: `%d`", len(plan.Handlers)),
			)
			lines = append(lines, tryHandlerHoverLines(plan.Handlers)...)
			// rules/errors/errorhandling.md — §16: unmatched failures of a
			// partial handler set propagate through the enclosing return.
			if plan.ResidualPropagates {
				lines = append(lines, "Unhandled errors: `propagated to "+lspTypeName(plan.EnclosingResultType)+"`")
				propagation := resolved
				propagation.EnclosingResultType = plan.EnclosingResultType
				lines = append(lines, errorIdentityHoverLines(propagation)...)
			}
		}
	default:
		lines = append(lines, "Failure handling: `propagated`")
		if len(resolved.Failures) > 0 {
			lines = append(lines, tryFailureSetHoverLines(resolved.Failures)...)
		} else {
			lines = append(lines, "Propagated error: `"+lspTypeName(resolved.ErrorType)+"`")
		}
		if resolved.TestBoundary {
			// rules/errors/errorhandling.md §41: the test invocation is the
			// boundary; it has no source-visible Result.
			lines = append(lines,
				"Propagation target: `test invocation`",
				"Err consumed by: `test failure (unexpected error)`",
			)
		} else {
			lines = append(lines,
				"Propagation target: `"+lspTypeName(resolved.EnclosingResultType)+"`",
				"Err consumed by: `enclosing function return`",
			)
		}
		lines = append(lines, errorIdentityHoverLines(resolved)...)
	}
	lines = append(lines, tryRemainingPanicHoverLines(analyzer, expression)...)

	return strings.Join(lines, "\n\n")
}

// tryRemainingPanicHoverLines shows the panic paths a try leaves in its
// protected expression; try converts only the failures it handles and never
// catches panic (errorhandling.md § 31). Nothing is shown when Sema has no
// fact or no panic path remains.
//
// Rules:
//   - rules/tooling/lsp.md — try hover "remaining panic effect"
//   - rules/analysis/effect_analysis.md — try removes only the supported panic path
func tryRemainingPanicHoverLines(analyzer *sema.Analyzer, expression *ast.TryExpression) []string {
	remaining, ok := analyzer.ResolvedTryRemainingPanicsOf(expression)
	if !ok || len(remaining) == 0 {
		return nil
	}
	lines := []string{"Remaining panic effect: `may panic` (try does not catch panic)"}
	for _, panic := range remaining {
		reasons := make([]string, 0, len(panic.PanicReasonIDs))
		for _, id := range panic.PanicReasonIDs {
			if definition, found := diagnostics.PanicReasonByID(id); found {
				reasons = append(reasons, definition.Name)
			}
		}
		reasonText := ""
		if len(reasons) > 0 {
			reasonText = ": `" + strings.Join(reasons, "`, `") + "`"
		}
		if panic.Callee != "" {
			lines = append(lines, fmt.Sprintf("- call to `%s` at line %d may panic%s", panic.Callee, panic.Source.Line, reasonText))
		} else {
			lines = append(lines, fmt.Sprintf("- `%s` at line %d%s", panic.Kind, panic.Source.Line, reasonText))
		}
	}
	return lines
}

// tryAssignmentHover presents the Sema-resolved contract of a fallible
// assignment `try place = value` at its try keyword: the error channel and
// whether failures propagate or reach local handlers.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment", §36 "LSP requirements"
//   - rules/tooling/lsp.md — "Hover"
func tryAssignmentHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	for _, node := range astNodesInProgram(program) {
		statement, ok := node.(*ast.TryAssignmentStatement)
		if !ok || !sameSourceToken(statement.Token, hovered) {
			continue
		}
		resolved, ok := analyzer.ResolvedTryAssignmentOf(statement)
		if !ok {
			return hoverResult{}, false
		}
		lines := []string{"### `try` assignment", "Error channel: `" + lspTypeName(resolved.ErrorType) + "`"}
		switch resolved.Kind {
		case sema.ResolvedTryAssignmentPropagation:
			target := "`" + lspTypeName(resolved.EnclosingResultType) + "`"
			if resolved.TestBoundary {
				target = "`test invocation`"
			}
			lines = append(lines,
				"Failure handling: `propagated`",
				"Propagation target: "+target,
			)
		default:
			coverage := "partial"
			if resolved.HandlerPlan.Exhaustive {
				coverage = "exhaustive"
			}
			lines = append(lines, "Failure handling: `local try handlers`", "Handler coverage: `"+coverage+"`")
			lines = append(lines, tryHandlerHoverLines(resolved.HandlerPlan.Handlers)...)
			if resolved.HandlerPlan.ResidualPropagates {
				lines = append(lines, "Unhandled errors: `propagated to "+lspTypeName(resolved.HandlerPlan.EnclosingResultType)+"`")
			}
		}
		return hoverResult{
			Contents: markupContent{Kind: "markdown", Value: strings.Join(lines, "\n\n")},
			Range:    tokenRange(text, statement.Token),
		}, true
	}
	return hoverResult{}, false
}
