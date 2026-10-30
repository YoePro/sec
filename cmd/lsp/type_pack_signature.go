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
	if !ok {
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
