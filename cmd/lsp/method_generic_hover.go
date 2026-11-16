package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// methodGenericCallHover presents the exact selected method signature and the
// compiler's separately ordered owner/method bindings. Declaration and generic
// parameter hovers continue through the ordinary definition-bound presenters.
// Rules: rules/tooling/lsp.md — §12(1),(3);
// rules/declarations/generics.md — §§9,18–20,29–30.
func methodGenericCallHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		call, ok := expression.(*ast.CallExpression)
		if !ok {
			continue
		}
		member, ok := call.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil || !sameSourceToken(member.Property.Token, token) {
			continue
		}
		fact, ok := analyzer.ResolvedMethodTypeArgumentsOf(call)
		if !ok {
			continue
		}
		target, ok := analyzer.ResolvedCallTarget(call)
		if !ok {
			continue
		}
		contents := functionHoverContents([]sema.Function{target.Function}, program, token.File)
		display := func(arguments []sema.MethodTypeArgument) string {
			values := make([]string, len(arguments))
			for index, argument := range arguments {
				values[index] = argument.Name + " = " + lspTypeName(argument.Type)
			}
			return strings.Join(values, ", ")
		}
		if len(fact.Owner) > 0 {
			contents += "\n\nOwner type arguments: `" + display(fact.Owner) + "`"
		}
		contents += "\n\nMethod type arguments: `" + display(fact.Method) + "`"
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
	}
	return hoverResult{}, false
}
