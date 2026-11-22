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
	// The cursor past every selected position, including in an open call
	// that so far resolves to the empty pack, gets the declared template.
	if !ok || active >= len(fact.Parameters) {
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
	key, owner, ok := typePackTemplateCallee(analyzer, call)
	if !ok {
		return signatureHelp{}, false
	}
	// A call Sema resolved to an ordinary function keeps that selection, and
	// an overload set with ordinary candidates is not reduced to its packs
	// (functions.md § 38A(4)(5)).
	if target, resolved := analyzer.ResolvedCallTarget(call); resolved && target.Function.GenericTypePack == "" {
		return signatureHelp{}, false
	}
	var templates []sema.Function
	for _, function := range analyzer.Functions()[key] {
		if function.ImplTarget != owner || !analyzer.FunctionVisibleFromSource(function, sourcePath) {
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
			Label:         "fn " + qualifiedTemplateName(owner, function.Name) + "[..." + function.GenericTypePack + "](" + strings.Join(labels, ", ") + ") " + lspTypeName(function.ReturnType),
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

// typePackTemplateCallee names the declared callables a call may select: a
// free function by its name, a static method by its owner type (Box[int].Make
// uses the owner Box) and an instance method by its receiver's resolved type.
// Rules: rules/declarations/functions.md — §§ 22-24, 38A(10)(3); rules/tooling/lsp.md — "Signature help".
func typePackTemplateCallee(analyzer *sema.Analyzer, call *ast.CallExpression) (key, owner string, ok bool) {
	switch callee := call.Callee.(type) {
	case *ast.Identifier:
		if callee == nil {
			return "", "", false
		}
		return callee.Value, "", true
	case *ast.MemberExpression:
		if callee == nil || callee.Property == nil {
			return "", "", false
		}
		object := callee.Object
		if indexed, generic := object.(*ast.IndexExpression); generic {
			object = indexed.Left
		}
		if typ, typed := analyzer.ResolvedTypeOf(callee.Object); typed {
			for typ.Kind == sema.ReferenceType && typ.Element != nil {
				typ = *typ.Element
			}
			// A closed generic owner such as Box[int] declares its methods on Box.
			if name, _, _ := strings.Cut(typ.Name, "["); name != "" {
				return name + "." + callee.Property.Value, name, true
			}
		}
		if name, isName := object.(*ast.Identifier); isName && name != nil {
			return name.Value + "." + callee.Property.Value, name.Value, true
		}
	}
	return "", "", false
}

// qualifiedTemplateName prefixes a method with its owner unless Sema's
// function name already carries it (static methods are named Owner.Method).
func qualifiedTemplateName(owner, name string) string {
	if owner == "" || strings.HasPrefix(name, owner+".") {
		return name
	}
	return owner + "." + name
}
