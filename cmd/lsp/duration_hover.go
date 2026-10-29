package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// quantityDurationHover presents Sema's quantity type and accepted duration
// conversion plan, without deriving unit coordinates, rounding or exactness.
// Callee hover selects the conversion; a literal selects its own quantity or
// contextual materialization. Imported AST nodes cannot match local offsets.
// Rules: rules/types/temporal.md — §§5.2–5.7;
// rules/types/units.md — "Quantity literals", "LSP requirements";
// rules/tooling/lsp.md — "Hover".
func quantityDurationHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		plan, converted := analyzer.DurationConversionPlanOf(expression)
		var literal *ast.QuantityLiteral
		var start lexer.Token
		rng := tokenRange(text, hovered)
		switch expression := expression.(type) {
		case *ast.CallExpression:
			if !converted || !expressionOwnsTypeToken(expression, hovered) {
				continue
			}
		case *ast.QuantityLiteral:
			literal, start = expression, expression.Token
		case *ast.PrefixExpression:
			if !converted {
				continue
			}
			literal, _ = expression.Right.(*ast.QuantityLiteral)
			if literal == nil {
				continue
			}
			start = expression.Token
		default:
			continue
		}
		if literal != nil {
			if hovered.File != start.File || hovered.ByteStart < start.ByteStart || hovered.ByteEnd > literal.EndToken.ByteEnd {
				continue
			}
			rng = lspRange{Start: tokenRange(text, start).Start, End: tokenRange(text, literal.EndToken).End}
		}
		typ := plan.Source
		if !converted {
			var known bool
			typ, known = analyzer.ResolvedTypeOf(literal)
			if !known || typ.Kind == sema.InvalidType {
				continue
			}
		}
		contents := "```sec\nquantity: " + sema.TypeDisplayName(typ) + "\n```"
		contents += unitDerivationHoverSuffix(analyzer, typ)
		if converted {
			contents += "\n\n" + durationPlanHoverContents(plan)
			if literal == nil {
				if protected, ok := tryExpressionHover(text, program, analyzer, hovered); ok {
					contents += "\n\n" + protected.Contents.Value
				}
			}
		}
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: rng}, true
	}
	return hoverResult{}, false
}

// durationPlanHoverContents formats only retained proof/rounding/result facts.
// A checked plan reports its first possible failure, not an exactness promise.
// Rules: rules/types/temporal.md — §§5.4–5.7;
// rules/types/units.md — "LSP requirements".
func durationPlanHoverContents(plan sema.DurationConversionPlan) string {
	form := "contextual"
	if plan.Explicit {
		form = "explicit"
	}
	lines := []string{"Duration conversion: `" + form + "`", "Proof: `" + string(plan.Proof) + "`"}
	if plan.NanosecondsPerUnit != nil {
		lines = append(lines, "Nanoseconds per unit: `"+plan.NanosecondsPerUnit.RatString()+"`")
	}
	rounding := "none (exact nanoseconds required)"
	if plan.Rounding != "" {
		rounding = "RoundingMode." + plan.Rounding
	}
	lines = append(lines, "Rounding: `"+rounding+"`")
	if plan.Nanoseconds != nil {
		lines = append(lines, "Nanoseconds: `"+plan.Nanoseconds.String()+"`")
	}
	if plan.Minimum != nil && plan.Maximum != nil {
		lines = append(lines, "Nanosecond range: `"+plan.Minimum.String()+" ..= "+plan.Maximum.String()+"`")
	}
	if plan.Proof == sema.DurationConversionChecked {
		lines = append(lines, "Result: `Result[duration, ConversionError]`", "First possible failure: `ConversionError."+string(plan.Failure)+"`")
	} else {
		lines = append(lines, "Result: `duration`")
	}
	return strings.Join(lines, "\n\n")
}
