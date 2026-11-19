package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// initializerSignatureHelp shows the compiler-selected init for completed
// constructions, or compiler-owned declaration previews for recovery/unresolved
// calls. The shared parser-owned call selector bounds this request, including
// nested calls and following statements; no editor overload inference occurs.
// Rules: rules/tooling/lsp.md — §§4.1(22),13, "Recovery nodes", "Unknown facts";
// rules/declarations/impl.md — §§15.4,15.6.
func initializerSignatureHelp(analyzer *sema.Analyzer, construction *ast.NewExpression, text string, open, offset int) (signatureHelp, bool) {
	active := activeConstructionParameter(text, open, offset)
	if resolved, ok := analyzer.ResolvedConstructionOf(construction); ok && construction.ArgumentsClose.Type == lexer.RPAREN {
		information := initializerSignatureInformation(resolved, false)
		return signatureHelp{Signatures: []signatureInformation{information}, ActiveParameter: initializerActiveParameter(active, len(information.Parameters))}, true
	}
	candidates := analyzer.ConstructionSignaturesOf(construction)
	if len(candidates) == 0 {
		return signatureHelp{}, false
	}
	help := signatureHelp{}
	for _, candidate := range candidates {
		help.Signatures = append(help.Signatures, initializerSignatureInformation(sema.ResolvedConstruction{
			Target: candidate.Target, ErrorType: candidate.ErrorType, Implicit: candidate.Implicit,
			Initializer: sema.Function{Parameters: candidate.Parameters},
		}, true))
	}
	// Pick a display row that has the cursor's argument position. This is
	// presentation only: no candidate is reported as semantically selected.
	for index, signature := range help.Signatures {
		if len(signature.Parameters) > len(help.Signatures[help.ActiveSignature].Parameters) {
			help.ActiveSignature = index
		}
		if active < len(signature.Parameters) {
			help.ActiveSignature = index
			break
		}
	}
	help.ActiveParameter = initializerActiveParameter(active, len(help.Signatures[help.ActiveSignature].Parameters))
	return help, true
}

// initializerSignatureInformation keeps parameter ownership and fallibility
// presentation shared between selected construction and declaration previews.
// Rules: rules/tooling/lsp.md — §§4.1(22),13(1)–(2).
func initializerSignatureInformation(resolved sema.ResolvedConstruction, preview bool) signatureInformation {
	labels := make([]string, 0, len(resolved.Initializer.Parameters))
	information := signatureInformation{}
	for _, parameter := range resolved.Initializer.Parameters {
		label := initializerParameterLabel(parameter)
		labels = append(labels, label)
		information.Parameters = append(information.Parameters, parameterInformation{Label: label})
	}
	information.Label = "init(" + strings.Join(labels, ", ") + ")"
	if preview && resolved.Implicit {
		information.Label = "implicit init()"
	}
	documentation := initializerConstructionDetails(resolved)
	if preview {
		documentation = "Initializer candidate; no overload selection is established.\n\n" + documentation
	}
	information.Documentation = markupContent{Kind: "markdown", Value: documentation}
	return information
}

// initializerActiveParameter keeps cursor information within a displayed
// declaration's bounds, including zero-argument implicit construction.
// Rules: rules/tooling/lsp.md — §13(1).
func initializerActiveParameter(active, count int) int {
	if count == 0 {
		return 0
	}
	if active >= count {
		return count - 1
	}
	return active
}
