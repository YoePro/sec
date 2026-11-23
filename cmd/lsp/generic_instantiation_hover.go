package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// genericInstantiationHover adds compiler-resolved parameter bindings at the
// exact instantiated use, including explicit argument tokens. It leaves
// template constraints intact and never infers or creates a specialization.
// Rules: rules/tooling/lsp.md — §§12.1(3),18(2);
// rules/declarations/generics.md — §§18–20,25–26,30.
func genericInstantiationHover(program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) string {
	matchesType := func(ref *ast.TypeReference) bool {
		return ref != nil && (sameSourceToken(ref.Token, token) || sameSourceToken(ref.NameToken, token) || sameSourceToken(ref.ReferentToken, token))
	}
	display := func(arguments []sema.GenericTypeArgument) string {
		parts := make([]string, len(arguments))
		for i, argument := range arguments {
			parts[i] = argument.Name + " = " + lspTypeName(argument.Type)
		}
		return "\n\nType arguments: `" + strings.Join(parts, ", ") + "`"
	}
	if typ, ok := analyzer.ResolvedSourceTypeAt(token.File, token.Line, token.Column); ok {
		if arguments, ok := analyzer.GenericTypeArgumentsOf(typ); ok {
			return display(arguments)
		}
	}
	binding, bound := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column)
	if !bound {
		binding, bound = analyzer.ResolvedBindingDeclaredAt(token.File, token.Line, token.Column)
	}
	if bound {
		if arguments, ok := analyzer.GenericTypeArgumentsOf(binding.Type); ok {
			return display(arguments)
		}
	}
	for _, expression := range astExpressionsInProgram(program) {
		call, ok := expression.(*ast.CallExpression)
		if !ok {
			continue
		}
		var callee lexer.Token
		switch owner := call.Callee.(type) {
		case *ast.Identifier:
			callee = owner.Token
		case *ast.MemberExpression:
			if owner.Property == nil {
				continue
			}
			callee = owner.Property.Token
			if sameSourceToken(callee, token) {
				if _, ok := analyzer.ResolvedMethodTypeArgumentsOf(call); ok {
					return "" // Existing method hover retains separate owner/method roles.
				}
			}
		default:
			continue
		}
		matches := sameSourceToken(callee, token)
		for _, argument := range call.GenericArguments {
			matches = matches || matchesType(argument)
		}
		if matches {
			if arguments, ok := analyzer.GenericCallTypeArgumentsOf(call); ok {
				return display(arguments)
			}
		}
	}
	for _, ref := range astTypeReferencesInProgram(program) {
		if len(ref.TypeArgs) == 0 {
			continue
		}
		matches := matchesType(ref)
		for _, argument := range ref.TypeArgs {
			matches = matches || matchesType(argument)
		}
		if matches {
			owner := ref.Token
			if ref.Ref {
				owner = ref.ReferentToken
			}
			if typ, ok := analyzer.ResolvedSourceTypeAt(owner.File, owner.Line, owner.Column); ok {
				if arguments, ok := analyzer.GenericTypeArgumentsOf(typ); ok {
					return display(arguments)
				}
			}
		}
	}
	return ""
}
