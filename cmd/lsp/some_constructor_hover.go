package main

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// someConstructorHover presents Sema's selected Option variant at its callee
// token. Ordinary resolved calls keep their normal function hover; no payload
// inference or overload selection is performed by the LSP.
// Rules: rules/types/types.md — "Contextual and self-inferred `Some(value)`";
// rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§11–15.
func someConstructorHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	if token.Lexeme != "Some" {
		return hoverResult{}, false
	}
	for _, expression := range astExpressionsInProgram(program) {
		call, ok := expression.(*ast.CallExpression)
		if !ok {
			continue
		}
		var callee *ast.Identifier
		switch value := call.Callee.(type) {
		case *ast.Identifier:
			callee = value
		case *ast.MemberExpression:
			callee = value.Property
		}
		if callee == nil || !sameSourceToken(callee.Token, token) {
			continue
		}
		if _, ordinary := analyzer.ResolvedCallTarget(call); ordinary {
			return hoverResult{}, false
		}
		fact, resolved := analyzer.ResolvedUnionConstructionOf(call)
		if !resolved || fact.UnionType.Name != "Option" || fact.VariantName != "Some" {
			return hoverResult{}, false
		}
		name := lspTypeName(fact.UnionType)
		contents := fmt.Sprintf("Compiler-known constructor: `%s.%s`\n\nSelected type: `%s`", name, fact.VariantName, name)
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
	}
	return hoverResult{}, false
}
