package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// omittedStructDefaultsHint shows the exact defaults of a valid source literal,
// in declaration order before its closing brace. Explicit and spread sources
// are excluded by the compiler's final-field plan. Synthetic literals have no
// real opening token and receive no duplicate hint. Preview bounds affect only
// presentation and never source, DefaultValueOf selection, or construction.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Hover" defaultable omitted fields;
// rules/types/default_values.md — A.7, A.11;
// rules/declarations/struct.md — §6 omitted fields, §9 spread.
func omittedStructDefaultsHint(analyzer *sema.Analyzer, text string, literal *ast.StructLiteral) (inlayHint, bool) {
	if literal == nil || !validDefinitionTokenForHint(literal.Open) {
		return inlayHint{}, false
	}
	plan, ok := analyzer.ResolvedStructLiteralPlanOf(literal)
	if !ok || !plan.FullyInitialized {
		return inlayHint{}, false
	}
	open := textPositionOffset(text, literal.Open.Line, literal.Open.Column)
	if open < 0 {
		return inlayHint{}, false
	}
	close := structLiteralClosingBrace(text, open)
	if close < 0 {
		return inlayHint{}, false
	}
	fields := []string{}
	omitted := 0
	for _, field := range plan.FinalFields {
		if field.SourceKind != sema.StructFieldSourceDefault {
			continue
		}
		omitted++
		if len(fields) == 8 {
			continue
		}
		value, ok := sema.ResolvedDefaultValueDisplay(field.FieldType, field.Default)
		if !ok {
			continue
		}
		fields = append(fields, field.FieldName+": "+boundedDefaultHintText(value, 80))
	}
	if len(fields) == 0 {
		return inlayHint{}, false
	}
	if omitted > len(fields) {
		fields = append(fields, "…")
	}
	label := boundedDefaultHintText("defaults: "+strings.Join(fields, ", "), 160)
	return inlayHint{Position: offsetPosition(text, close), Label: label, PaddingLeft: true, PaddingRight: true}, true
}

// boundedDefaultHintText limits a display preview by Unicode scalar count.
// Rules: rules/tooling/lsp.md — "Inlay hints", compact non-editing presentation.
func boundedDefaultHintText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

// structLiteralClosingBrace follows lexical delimiters so comments and string
// contents cannot displace a literal's closing-brace hint.
// Rules: rules/foundations/lexical_structure.md — §5 comments, string literals;
// rules/tooling/lsp.md — source-position mapping and inlay hints.
func structLiteralClosingBrace(text string, open int) int {
	if open < 0 || open >= len(text) || text[open] != '{' {
		return -1
	}
	scanner := lexer.New(text[open:])
	depth := 0
	for {
		token := scanner.NextToken()
		switch token.Type {
		case lexer.EOF:
			return -1
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
			if depth == 0 {
				return open + token.ByteStart
			}
		}
	}
}
