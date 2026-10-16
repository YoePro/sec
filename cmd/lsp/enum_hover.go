package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// enumVariantHoverContents presents only an enum member Sema resolved to its
// declaration token, attaching the parser-owned documentation for that member.
// Rules: rules/tooling/lsp.md — "Hover";
// rules/concurrency/atomics.md — §43(1) (MemoryOrder variants).
func enumVariantHoverContents(analyzer *sema.Analyzer, program *ast.Program, definition lexer.Token) (string, bool) {
	for _, typ := range analyzer.Types() {
		if typ.Kind != sema.EnumType {
			continue
		}
		for _, name := range typ.EnumValues {
			value := typ.EnumConsts[name]
			if !sameSourceToken(value.Token, definition) {
				continue
			}
			contents := "```sec\n" + lspTypeName(typ) + "." + name + "\n```"
			for _, attachment := range program.Documentation {
				member, ok := attachment.Declaration.(*ast.EnumValue)
				if ok && member != nil && member.Name != nil && sameSourceToken(member.Name.Token, definition) {
					contents += "\n\n" + documentationCommentText(attachment.Comments)
				}
			}
			return contents, true
		}
	}
	return "", false
}

// enumTypeHoverContents renders the closed variant set retained by Sema. Core
// documentation stays attached to its real declaration rather than a second
// hand-maintained description of the compiler-known enum.
// Rules: rules/tooling/lsp.md — "Hover";
// rules/concurrency/atomics.md — §§5,43(1).
func enumTypeHoverContents(typ sema.Type, program *ast.Program) string {
	header := "enum " + typ.Name + genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints)
	// Rules: rules/declarations/enums.md — §3 "Declaration forms";
	// rules/concurrency/mutex.md — §7(1–3).
	if typ.ErrorAssignable {
		header += " error"
	}
	contents := "```sec\n" + header + " {\n    " + strings.Join(typ.EnumValues, "\n    ") + "\n}\n```"
	for _, attachment := range program.Documentation {
		declaration, ok := attachment.Declaration.(*ast.EnumDeclaration)
		if !ok || declaration == nil || declaration.Name == nil || declaration.Name.Value != typ.Name {
			continue
		}
		// Match canonical member provenance, not a same-spelled enum from
		// another declaration/module in a recovered program.
		if len(declaration.Values) > 0 && declaration.Values[0] != nil && declaration.Values[0].Name != nil && len(typ.EnumValues) > 0 && sameSourceToken(declaration.Values[0].Name.Token, typ.EnumConsts[typ.EnumValues[0]].Token) {
			contents += "\n\n" + documentationCommentText(attachment.Comments)
		}
	}
	return contents
}
