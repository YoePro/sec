package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// typePackSignatureHelp projects Sema's selected concrete receiving parameters;
// unresolved calls keep the existing fallback rather than LSP-owned inference.
// Rules: rules/declarations/functions.md §38A(10)(3);
// rules/tooling/lsp.md — "Signature help", "Unknown facts".
func typePackSignatureHelp(analyzer *sema.Analyzer, call *ast.CallExpression, active int) (signatureHelp, bool) {
	fact, ok := analyzer.ResolvedTypePackCallOf(call)
	if !ok || len(fact.Parameters) > 0 && active >= len(fact.Parameters) {
		return signatureHelp{}, false
	}
	target, ok := analyzer.ResolvedCallTarget(call)
	if !ok {
		return signatureHelp{}, false
	}
	labels := make([]string, len(fact.Parameters))
	parameters := make([]parameterInformation, len(fact.Parameters))
	for i, parameter := range fact.Parameters {
		name := parameter.Name
		if i >= fact.FixedCount {
			name = fmt.Sprintf("%s[%d]", name, i-fact.FixedCount)
		}
		if parameter.Consuming {
			name = "-> " + name
		}
		labels[i] = name + ": " + typePackReceivingTypeName(parameter.Type)
		parameters[i] = parameterInformation{Label: labels[i]}
	}
	if len(parameters) == 0 {
		active = 0
	} else if active >= len(parameters) {
		active = len(parameters) - 1
	}
	types := make([]string, len(fact.Types))
	for i, typ := range fact.Types {
		types[i] = lspTypeName(typ)
	}
	return signatureHelp{
		Signatures: []signatureInformation{{
			Label:         "fn " + target.Function.Name + "(" + strings.Join(labels, ", ") + ") " + lspTypeName(target.Function.ReturnType),
			Parameters:    parameters,
			Documentation: markupContent{Kind: "markdown", Value: "Inferred type pack: `" + fact.Name + " = (" + strings.Join(types, ", ") + ")`"},
		}},
		ActiveParameter: active,
	}, true
}

// typePackReceivingTypeName renders the concrete referent from the resolved
// parameter instead of the reference's retained template display name.
// Rules: rules/declarations/functions.md §38A(3),(10)(3).
func typePackReceivingTypeName(typ sema.Type) string {
	if typ.Kind != sema.ReferenceType || typ.Element == nil {
		return lspTypeName(typ)
	}
	mode := "ref "
	if typ.ReferenceMutable {
		mode += "mut "
	}
	return mode + lspTypeName(*typ.Element)
}

// typePackTemplateSignatureHelp shows the declared pack signature while a call
// is incomplete or unresolved, or the cursor is past the positions Sema has
// selected: `fn Trace[...Types](prefix: string, values: ...ref Types) void`,
// with the pack parameter active for every trailing position. It projects
// Sema's declared templates and invents no LSP-owned inference.
// Rules: rules/declarations/functions.md — § 38A(10)(3); rules/tooling/lsp.md — "Signature help", "Unknown facts".
func typePackTemplateSignatureHelp(analyzer *sema.Analyzer, call *ast.CallExpression, active int, sourcePath string) (signatureHelp, bool) {
	callee, ok := call.Callee.(*ast.Identifier)
	if !ok || callee == nil {
		return signatureHelp{}, false
	}
	// A call Sema resolved to an ordinary function keeps that selection, and
	// an overload set with ordinary candidates is not reduced to its packs
	// (functions.md § 38A(4)(5)).
	if target, resolved := analyzer.ResolvedCallTarget(call); resolved && target.Function.GenericTypePack == "" {
		return signatureHelp{}, false
	}
	var templates []sema.Function
	for _, function := range analyzer.Functions()[callee.Value] {
		if function.ImplTarget != "" || !analyzer.FunctionVisibleFromSource(function, sourcePath) {
			continue
		}
		if function.GenericTypePack == "" {
			return signatureHelp{}, false
		}
		templates = append(templates, function)
	}
	if len(templates) == 0 {
		return signatureHelp{}, false
	}
	help := signatureHelp{}
	for _, function := range templates {
		labels := make([]string, len(function.Parameters))
		parameters := make([]parameterInformation, len(function.Parameters))
		for i, parameter := range function.Parameters {
			name := parameter.Name
			if parameter.Consuming {
				name = "-> " + name
			}
			labels[i] = name + ": " + typePackTemplateParameterType(parameter)
			parameters[i] = parameterInformation{Label: labels[i]}
		}
		help.Signatures = append(help.Signatures, signatureInformation{
			Label:         "fn " + function.Name + "[..." + function.GenericTypePack + "](" + strings.Join(labels, ", ") + ") " + lspTypeName(function.ReturnType),
			Parameters:    parameters,
			Documentation: markupContent{Kind: "markdown", Value: "Type pack `" + function.GenericTypePack + "` is inferred from the trailing arguments once the call is complete."},
		})
	}
	fixed := len(templates[0].Parameters) - 1
	if active > fixed {
		active = fixed
	}
	if active < 0 {
		active = 0
	}
	help.ActiveParameter = active
	return help, true
}

// typePackTemplateParameterType spells a template parameter as declared: the
// trailing pack as `...Types`, `...ref Types` or `...ref mut Types`.
// Rules: rules/declarations/functions.md — § 38A(3).
func typePackTemplateParameterType(parameter sema.FunctionParameter) string {
	if !parameter.Variadic {
		return lspTypeName(parameter.Type)
	}
	typ := parameter.Type
	if typ.Kind == sema.VariadicPackType && typ.Element != nil {
		typ = *typ.Element
	}
	mode := ""
	if typ.Kind == sema.ReferenceType && typ.Element != nil {
		mode = "ref "
		if typ.ReferenceMutable {
			mode = "ref mut "
		}
		typ = *typ.Element
	}
	return "..." + mode + typ.Name
}
