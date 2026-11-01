package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
)

// interfaceMethodLabel presents the resolved requirement's receiver mode,
// parameters and exact error channel. It never specializes a template itself.
// Rules: rules/declarations/interfaces.md — §§3,6,9; MD-120 §10(7).
func interfaceMethodLabel(method sema.Function) (string, []parameterInformation) {
	mode := ""
	if method.Static {
		mode = "static "
	} else if method.ReceiverMutable {
		mode = "mut "
	}
	labels := make([]string, len(method.Parameters))
	params := make([]parameterInformation, len(method.Parameters))
	for i, p := range method.Parameters {
		labels[i] = initializerParameterLabel(p)
		params[i] = parameterInformation{Label: labels[i]}
	}
	return mode + "fn " + method.Name + "(" + strings.Join(labels, ", ") + ") " + lspTypeName(method.ReturnType), params
}

// interfaceHoverContents projects a declaration or concrete borrowed-view type,
// preserving its exact arguments and substituted requirement signatures.
// Rules: rules/tooling/lsp.md — "Hover"; MD-120 §§6,10(7).
func interfaceHoverContents(typ sema.Type) string {
	name := lspTypeName(typ)
	if len(typ.TypeArgs) == 0 {
		name += genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints)
	}
	lines := []string{"interface " + name + " {"}
	for _, method := range typ.InterfaceMethods {
		label, _ := interfaceMethodLabel(method)
		lines = append(lines, "    "+label)
	}
	lines = append(lines, "}")
	return "```sec\n" + strings.Join(lines, "\n") + "\n```"
}

// interfaceSourceHover handles source type positions and requirement declarations
// before name-only fallbacks. An explicit error root is never a value binding
// named error in some other body. Generic parameters retain their existing facts.
// Rules: rules/tooling/lsp.md — "Hover", "One semantic source of truth";
// MD-120 §§3(3),6,10(7).
func interfaceSourceHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	if constraint, ok := analyzer.GenericConstraintAt(token); ok && constraint.Interface.Kind == sema.ErrorRootType {
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: "```sec\n" + constraint.Parameter + ": error\n```\n\nCompile-time error-category requirement. The type parameter remains unknown; this is not an interface or implicit erasure to the open error root. It guarantees no concrete variants, copying or discardability."}, Range: tokenRange(text, token)}, true
	}
	if typ, ok := analyzer.ResolvedSourceTypeAt(token.File, token.Line, token.Column); ok {
		contents := ""
		if typ.Kind == sema.InterfaceType {
			contents = interfaceHoverContents(typ)
		}
		if typ.Kind == sema.ErrorRootType {
			contents = "```sec\nerror\n```\n\nOpen error root. This explicit argument is distinct from a constrained unknown error parameter and from a concrete error type; it does not erase an instantiated interface's error channel."
		}
		if contents != "" {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: tokenRange(text, token)}, true
		}
	}
	owner, hasOwner := interfaceRequirementOwner(program, token)
	for _, typ := range analyzer.Types() {
		if typ.Kind != sema.InterfaceType || len(typ.TypeArgs) != 0 {
			continue
		}
		if declaration, ok := analyzer.ResolvedTypeDeclarationLocation(typ); ok && sameSourceToken(declaration, token) {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: interfaceHoverContents(typ)}, Range: tokenRange(text, token)}, true
		}
		declaration, known := analyzer.ResolvedTypeDeclarationLocation(typ)
		if !hasOwner || !known || !sameSourceToken(declaration, owner) {
			continue
		}
		for _, method := range typ.InterfaceMethods {
			if sameSourceToken(method.Token, token) {
				label, _ := interfaceMethodLabel(method)
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: "```sec\n" + label + "\n```\n\nInterface requirement of `" + typ.Name + genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints) + "`."}, Range: tokenRange(text, token)}, true
			}
		}
	}
	return hoverResult{}, false
}

// interfaceCallHover uses only Sema's successful interface dispatch selection;
// it does not display a same-named concrete method or the generic declaration
// when the selected receiver fixes a concrete error channel.
// Rules: rules/tooling/lsp.md — "Hover"; MD-120 §§6,10(7),11(6).
func interfaceCallHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		call, ok := expression.(*ast.CallExpression)
		if !ok {
			continue
		}
		member, ok := call.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil || !sameSourceToken(member.Property.Token, token) {
			continue
		}
		target, ok := analyzer.ResolvedCallTarget(call)
		if !ok || target.Kind != sema.ResolvedInterfaceMethodCall {
			continue
		}
		label, _ := interfaceMethodLabel(target.Function)
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: "```sec\n" + label + "\n```\n\nSelected interface requirement; the error channel is the receiver's exact specialization."}, Range: tokenRange(text, token)}, true
	}
	return hoverResult{}, false
}

// interfaceSignatureHelp presents the already selected requirement while the
// cursor is inside its argument list. Unknown/invalid calls claim no selection.
// Rules: rules/tooling/lsp.md — "Signature help"; MD-120 §11(6).
func interfaceSignatureHelp(analyzer *sema.Analyzer, call *ast.CallExpression, active int) (signatureHelp, bool) {
	target, ok := analyzer.ResolvedCallTarget(call)
	if !ok || target.Kind != sema.ResolvedInterfaceMethodCall {
		return signatureHelp{}, false
	}
	label, params := interfaceMethodLabel(target.Function)
	if len(params) == 0 {
		active = 0
	} else if active >= len(params) {
		active = len(params) - 1
	}
	return signatureHelp{Signatures: []signatureInformation{{Label: label, Parameters: params, Documentation: markupContent{Kind: "markdown", Value: "Selected interface requirement with the receiver's exact type arguments."}}}, ActiveParameter: active}, true
}

// interfaceCompletionMethods projects the instantiated interface requirement
// list; shared views do not gain mutable receiver capability. Static members
// stay in the type namespace. It does not search concrete implementations.
// Rules: rules/declarations/interfaces.md — §§3,7.1,9;
// rules/tooling/lsp.md — "Completion"; MD-120 §6.
func interfaceCompletionMethods(typ sema.Type, static bool) []sema.Function {
	shared := typ.Kind == sema.ReferenceType && !typ.ReferenceMutable
	if typ.Kind == sema.ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	if typ.Kind != sema.InterfaceType && typ.Kind != sema.GenericType {
		return nil
	}
	methods := []sema.Function{}
	for _, method := range typ.InterfaceMethods {
		if method.Static == static && !(shared && method.ReceiverMutable) {
			methods = append(methods, method)
		}
	}
	return methods
}

// interfaceRequirementOwner locates the parser-owned declaration enclosing a
// method token. Inherited copies in Sema's types cannot claim that declaration;
// all signatures and constraints still come from the matched compiler type.
// Rules: rules/declarations/interfaces.md — §§5,6; rules/tooling/lsp.md — "Hover".
func interfaceRequirementOwner(program *ast.Program, token lexer.Token) (lexer.Token, bool) {
	for _, statement := range program.Statements {
		declaration, ok := statement.(*ast.InterfaceDeclaration)
		if !ok || declaration.Name == nil {
			continue
		}
		for _, method := range declaration.Methods {
			if method != nil && method.Name != nil && sameSourceToken(method.Name.Token, token) {
				return declaration.Name.Token, true
			}
		}
	}
	return lexer.Token{}, false
}
