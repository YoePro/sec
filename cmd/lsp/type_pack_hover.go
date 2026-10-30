package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
)

// typePackDeclarationHover projects a recognized template's exact declaration
// signature, including an impl-qualified method. It does not select an overload
// or infer a concrete pack at a call site while expansion remains unavailable.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §10(3);
// rules/tooling/lsp.md — "Hover".
func typePackDeclarationHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	for _, overloads := range analyzer.Functions() {
		for _, function := range overloads {
			if function.GenericTypePack != "" && sameSourceToken(function.Token, token) {
				contents := functionHoverContents([]sema.Function{function}, program, token.File)
				contents += "\n\nType-pack declaration; shared-pack bodies with no pack use or only Len reads and direct-call inference are supported. Pack-element body expansion and lowering remain unavailable."
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
			}
		}
	}
	return hoverResult{}, false
}

// typePackCallHover presents the selected ordered sequence and receiving modes
// directly from Sema; it never infers types or resolves overloads in the LSP.
// Rules: rules/declarations/functions.md §38A(10)(3); rules/tooling/lsp.md — "Hover".
func typePackCallHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
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
		fact, ok := analyzer.ResolvedTypePackCallOf(call)
		if !ok {
			continue
		}
		types := make([]string, len(fact.Types))
		for i, typ := range fact.Types {
			types[i] = lspTypeName(typ)
		}
		modes := make([]string, 0, len(fact.Types))
		for _, parameter := range fact.Parameters[fact.FixedCount:] {
			modes = append(modes, typePackReceivingTypeName(parameter.Type))
		}
		contents := ""
		if target, resolved := analyzer.ResolvedCallTarget(call); resolved {
			contents = functionHoverContents([]sema.Function{target.Function}, program, token.File) + "\n\n"
		}
		contents += "Inferred type pack: `" + fact.Name + " = (" + strings.Join(types, ", ") + ")`"
		contents += "\n\nPack receiving types: `(" + strings.Join(modes, ", ") + ")`"
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
	}
	return hoverResult{}, false
}
