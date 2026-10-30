package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// errorVariantHoverContents presents the resolved error union variant and its
// available payload. Declaration-token identity comes from Sema, so contextual
// and qualified patterns share presentation without a variant-name lookup.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/errors/errorhandling.md — §15.3;
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§4,5.
func errorVariantHoverContents(program *ast.Program, analyzer *sema.Analyzer, use, definition lexer.Token) (string, bool) {
	types := []sema.Type{}
	if channel, ok := errorPatternHoverChannel(program, analyzer, use); ok && channel.Kind != sema.ErrorRootType {
		// A specialized error's payload must come from this exact channel,
		// not its generic declaration in the exposed type table.
		types = append(types, channel)
	} else {
		for _, typ := range analyzer.Types() {
			types = append(types, typ)
		}
	}
	for _, typ := range types {
		if typ.Kind != sema.UnionType || !typ.ErrorAssignable {
			continue
		}
		for _, variant := range typ.UnionVariants {
			if !sameSourceToken(variant.Token, definition) {
				continue
			}
			label := lspTypeName(typ) + "." + variant.Name
			if variant.Payload != nil {
				label += "(" + lspTypeName(*variant.Payload) + ")"
			} else if len(variant.PayloadFields) > 0 {
				fields := []string{}
				for _, field := range variant.PayloadFields {
					fields = append(fields, field.Name+": "+lspTypeName(field.Type))
				}
				label += " { " + strings.Join(fields, ", ") + " }"
			}
			return "```sec\n" + label + "\n```", true
		}
	}
	return "", false
}

// errorPatternHoverChannel connects the selected variant token to its owning
// Err pattern, then queries the same Sema channel used by completion. This
// retains concrete generic arguments without inferring types from spelling.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/errors/errorhandling.md — §§15.3,27.2b.
func errorPatternHoverChannel(program *ast.Program, analyzer *sema.Analyzer, use lexer.Token) (sema.Type, bool) {
	for _, node := range astNodesInProgram(program) {
		var owner lexer.Token
		var variant *ast.Identifier
		switch node := node.(type) {
		case *ast.MatchPattern:
			if node.Name != "Err" {
				continue
			}
			owner, variant = node.Token, node.ContextualErrorVariant
			if node.ErrorVariant != nil {
				variant = node.ErrorVariant.Property
			}
		case *ast.ErrExpression:
			owner = node.Token
			if member, ok := node.Value.(*ast.MemberExpression); ok {
				variant = member.Property
			}
		}
		if variant != nil && sameSourceToken(variant.Token, use) {
			return errorPatternChannelAt(program, analyzer, owner)
		}
	}
	return sema.Type{}, false
}
