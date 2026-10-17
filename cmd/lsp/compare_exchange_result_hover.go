package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// compareExchangeResultHoverContents renders the canonical associated-value
// enum from compiler facts, retaining the declaration's source documentation.
// Rules: rules/concurrency/atomics.md — §§6,43(2),44(3–5).
func compareExchangeResultHoverContents(typ sema.Type, program *ast.Program) (string, bool) {
	typ, ok := sema.CompareExchangeResultType(typ)
	if !ok {
		return "", false
	}
	name := lspTypeName(typ) + genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints)
	var lines []string
	for _, variant := range typ.UnionVariants {
		line := variant.Name
		if variant.Payload != nil {
			line += "(" + lspTypeName(*variant.Payload) + ")"
		}
		lines = append(lines, line)
	}
	contents := "```sec\nenum " + name + " {\n    " + strings.Join(lines, "\n    ") + "\n}\n```\n\nNormal compare-exchange outcomes; this is not an error type. NotExchanged carries the value observed by the same operation."
	if program != nil {
		for _, attachment := range program.Documentation {
			declaration, ok := attachment.Declaration.(*ast.EnumDeclaration)
			if ok && declaration.Name != nil && sameSourceToken(declaration.Name.Token, typ.DeclarationToken) {
				contents += "\n\n" + documentationCommentText(attachment.Comments)
			}
		}
	}
	return contents, true
}

// compareExchangeVariantContents presents a resolved variant's concrete payload
// and its real core documentation, without interpreting variant spellings.
// Rules: rules/concurrency/atomics.md — §§6(3–6),43(2),44(4–5).
func compareExchangeVariantContents(typ sema.Type, index uint32, program *ast.Program) (string, bool) {
	typ, ok := sema.CompareExchangeResultType(typ)
	if !ok || int(index) >= len(typ.UnionVariants) {
		return "", false
	}
	variant := typ.UnionVariants[index]
	name := lspTypeName(typ) + genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints) + "." + variant.Name
	if variant.Payload != nil {
		name += "(" + lspTypeName(*variant.Payload) + ")"
	}
	contents := "```sec\n" + name + "\n```"
	for _, attachment := range program.Documentation {
		declaration, ok := attachment.Declaration.(*ast.EnumValue)
		if ok && declaration.Name != nil && sameSourceToken(declaration.Name.Token, variant.Token) {
			contents += "\n\n" + documentationCommentText(attachment.Comments)
		}
	}
	return contents, true
}

// compareExchangePatternToken locates a parser-owned variant name, without
// deciding its type or identity; those come from the resolved match plan.
// Rules: rules/concurrency/atomics.md — §44; rules/tooling/lsp.md — Hover.
func compareExchangePatternToken(expr ast.Expression) lexer.Token {
	switch value := expr.(type) {
	case *ast.Identifier:
		return value.Token
	case *ast.MemberExpression:
		if value.Property != nil {
			return value.Property.Token
		}
	case *ast.CallExpression:
		return compareExchangePatternToken(value.Callee)
	}
	return lexer.Token{}
}

// compareExchangeResultHoverAt consumes constructor and match facts at a
// variant use, preserving the instantiated payload rather than showing open T.
// Rules: rules/concurrency/atomics.md — §§43(2),44(3–5).
func compareExchangeResultHoverAt(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	template, canonical := sema.CompareExchangeResultType(analyzer.Types()["CompareExchangeResult"])
	if !canonical {
		return hoverResult{}, false
	}
	candidate := false
	for _, variant := range template.UnionVariants {
		if token.Lexeme == variant.Name {
			candidate = true
			break
		}
	}
	if !candidate {
		return hoverResult{}, false
	}
	for _, node := range astNodesInProgram(program) {
		if expression, ok := node.(ast.Expression); ok && sameSourceToken(compareExchangePatternToken(expression), token) {
			if resolved, found := analyzer.ResolvedUnionConstructionOf(expression); found {
				if contents, known := compareExchangeVariantContents(resolved.UnionType, resolved.VariantIndex, program); known {
					return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
				}
			}
		}
		if match, ok := node.(*ast.MatchExpression); ok {
			plan, found := analyzer.ResolvedMatchPlanOf(match)
			if !found {
				continue
			}
			for _, arm := range plan.Arms {
				if arm.SourceIndex < 0 || arm.SourceIndex >= len(match.Arms) || match.Arms[arm.SourceIndex].Pattern == nil || !sameSourceToken(match.Arms[arm.SourceIndex].Pattern.NameToken, token) {
					continue
				}
				if contents, known := compareExchangeVariantContents(plan.SubjectType, arm.UnionVariantIndex, program); known {
					return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
				}
			}
		}
	}
	return hoverResult{}, false
}
