package main

import (
	"sec/internal/ast"
	"sec/internal/sema"
	"strings"
)

// signatureHelpForSource projects canonical call contracts from one analyzed
// snapshot, preserving initializer help and adding raw-pointer operation help.
// Rules: rules/tooling/lsp.md — "Signature help", "Snapshots";
// rules/compiler/compiler_known_members.md — "LSP";
// rules/memory/raw_pointers.md — §§5.2,35(2–6).
func signatureHelpForSource(uri string, text string, pos position, overlays ...sourceOverlay) (signatureHelp, bool) {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return signatureHelp{}, false
	}
	offset := lineCharToOffset(text, pos.Line, pos.Character)
	if offset < 0 {
		return signatureHelp{}, false
	}
	path := pathFromURI(uri)
	program, analyzer, release := cachedLSPAnalysis(uri, text, program, firstSourceOverlay(overlays))
	defer release()
	if call, found := signatureCallAtOffset(program, path, text, offset); found {
		if expression, ok := call.expression.(*ast.CallExpression); ok {
			if help, ok := typePackSignatureHelp(analyzer, expression, activeConstructionParameter(text, call.open, offset)); ok {
				return help, true
			}
			if member, ok := expression.Callee.(*ast.MemberExpression); ok && member != nil && member.Property != nil {
				known, found := analyzer.CompilerKnownMemberAt(member.Property.Token.File, member.Property.Token.Line, member.Property.Token.Column)
				if found && strings.HasPrefix(known.ID, "CKM-RAWPTR-") {
					parameters := make([]parameterInformation, 0, len(known.Parameters))
					for _, parameter := range known.Parameters {
						parameters = append(parameters, parameterInformation{Label: initializerParameterLabel(parameter)})
					}
					active := activeConstructionParameter(text, call.open, offset)
					if len(parameters) == 0 {
						active = 0
					} else if active >= len(parameters) {
						active = len(parameters) - 1
					}
					return signatureHelp{Signatures: []signatureInformation{{Label: compilerKnownSignatureLabel(known), Documentation: compilerKnownMemberHover(lspRange{}, known).Contents, Parameters: parameters}}, ActiveParameter: active}, true
				}
			}
		}
		if _, constructor := call.expression.(*ast.NewExpression); !constructor {
			return signatureHelp{}, false
		}
	}
	return initializerSignatureHelp(program, analyzer, path, text, offset)
}

// initializerSignatureHelp preserves canonical new/init parameter presentation.
// Rules: rules/tooling/lsp.md — "Lifecycle init and new", "Signature help".
func initializerSignatureHelp(program *ast.Program, analyzer *sema.Analyzer, path, text string, offset int) (signatureHelp, bool) {
	construction, openOffset, closeOffset, found := constructionAtOffset(program, path, text, offset)
	if !found || offset <= openOffset || offset > closeOffset {
		return signatureHelp{}, false
	}
	resolved, ok := analyzer.ResolvedConstructionOf(construction)
	if !ok {
		return signatureHelp{}, false
	}
	parameters := resolved.Initializer.Parameters
	labels := make([]string, 0, len(parameters))
	information := make([]parameterInformation, 0, len(parameters))
	for _, parameter := range parameters {
		label := initializerParameterLabel(parameter)
		labels = append(labels, label)
		information = append(information, parameterInformation{Label: label})
	}
	activeParameter := activeConstructionParameter(text, openOffset, offset)
	if len(parameters) == 0 {
		activeParameter = 0
	} else if activeParameter >= len(parameters) {
		activeParameter = len(parameters) - 1
	}
	documentation := initializerConstructionDetails(resolved)
	return signatureHelp{
		Signatures: []signatureInformation{{
			Label:         "init(" + strings.Join(labels, ", ") + ")",
			Documentation: markupContent{Kind: "markdown", Value: documentation},
			Parameters:    information,
		}},
		ActiveParameter: activeParameter,
	}, true
}
