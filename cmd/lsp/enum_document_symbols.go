package main

import "sec/internal/ast"

// enumDocumentSymbol projects the parser's real enum/member boundaries. The
// outline contains its children and complete initializers, including default
// markers and decoded strings, without guessing a closing brace in bad source.
// Rules: rules/declarations/enums.md — §§5,8.2,18;
// rules/tooling/lsp.md — "Document symbols".
func enumDocumentSymbol(text string, declaration *ast.EnumDeclaration) (documentSymbol, bool) {
	if declaration == nil || declaration.Name == nil {
		return documentSymbol{}, false
	}
	symbol := namedDocumentSymbol(text, declaration.Name.Value, "enum", 10, declaration.Token, declaration.Name.Token)
	for _, member := range declaration.Values {
		if member == nil || member.Name == nil {
			continue
		}
		child := namedDocumentSymbol(text, member.Name.Value, "enum member", 22, member.Token, member.Name.Token)
		if member.EndToken.Line > 0 {
			child.Range = rangeContaining(child.Range, tokenRange(text, member.EndToken))
		}
		symbol.Children = append(symbol.Children, child)
		symbol.Range = rangeContaining(symbol.Range, child.Range)
	}
	if declaration.BodyClose.Line > 0 {
		symbol.Range = rangeContaining(symbol.Range, tokenRange(text, declaration.BodyClose))
	}
	return symbol, true
}
