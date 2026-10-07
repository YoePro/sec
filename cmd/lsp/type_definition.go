package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// typeDefinitionsForSource implements textDocument/typeDefinition as a thin
// projection of Sema's resolved expression/binding types and declaration
// provenance. It deliberately performs no spelling-based type lookup.
//
// Rules:
//   - rules/tooling/lsp.md — "Navigation and references" and A.15
//   - rules/foundations/names_scopes_visibility.md — resolved symbol identity
func typeDefinitionsForSource(uri string, text string, pos position, overlays ...sourceOverlay) (locations []location) {
	defer func() {
		if recover() != nil {
			locations = nil
		}
	}()

	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	path := pathFromURI(uri)
	overlay := firstSourceOverlay(overlays)
	prepareProgramForLSP(program, path, overlay)
	analyzer := newLSPAnalyzer(uri, program)
	analyzer.Analyze(program)

	use, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return nil
	}
	typ, ok := resolvedTypeAtSourceToken(program, analyzer, use)
	if !ok {
		return nil
	}
	definition, ok := analyzer.ResolvedTypeDeclarationLocation(typ)
	if !ok {
		return nil
	}
	definitionURI := uri
	if definition.File != "" {
		definitionURI = uriFromPath(definition.File)
	}
	definitionText := sourceTextForToken(uri, text, overlay, definition)
	return []location{{URI: definitionURI, Range: definitionTokenRange(definitionText, definition)}}
}

func resolvedTypeAtSourceToken(program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (sema.Type, bool) {
	if binding, ok := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column); ok {
		return binding.Type, true
	}

	// Parent expressions occur before their children in the shared AST walk.
	// This makes a callable token in Build() resolve to the call's result type,
	// while a plain identifier still resolves to its own expression type.
	for _, expression := range astExpressionsInProgram(program) {
		if !expressionOwnsTypeToken(expression, token) {
			continue
		}
		if typ, ok := analyzer.ResolvedTypeOf(expression); ok {
			return typ, true
		}
	}

	// A cursor directly on a type reference already binds to the declaration.
	// Match that compiler-owned declaration token back to its resolved Type so
	// aliases and qualified imported identities keep their semantic identity.
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
	for _, typ := range analyzer.Types() {
		declaration, ok := analyzer.ResolvedTypeDeclarationLocation(typ)
		if !ok {
			continue
		}
		for _, definition := range definitions {
			if sameSourceToken(declaration, definition) {
				return typ, true
			}
		}
	}
	return sema.Type{}, false
}

func expressionOwnsTypeToken(expression ast.Expression, token lexer.Token) bool {
	switch expression := expression.(type) {
	case *ast.Identifier:
		return sameSourceToken(expression.Token, token)
	case *ast.MemberExpression:
		return expression.Property != nil && sameSourceToken(expression.Property.Token, token)
	case *ast.CallExpression:
		if expression.Function != nil && sameSourceToken(expression.Function.Token, token) {
			return true
		}
		member, ok := expression.Callee.(*ast.MemberExpression)
		return ok && member.Property != nil && sameSourceToken(member.Property.Token, token)
	case *ast.NewExpression:
		return expression.Type != nil && sameSourceToken(expression.Type.Token, token)
	case *ast.ConversionExpression:
		return expression.Type != nil && sameSourceToken(expression.Type.Token, token)
	default:
		return false
	}
}
