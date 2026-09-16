package sema

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateNominalTypeName checks the declaration's final name component after
// a visibility prefix. Imported and nested declarations may already have a
// lowercase module or owner prefix in their semantic name; that prefix is not
// part of the declared type's capitalization. Invalid names remain registered
// so analysis can continue without losing the declaration.
//
// Rules: rules/foundations/names_scopes_visibility.md — §6 Nested declarations,
// §13 Type naming.
func (a *Analyzer) validateNominalTypeName(name *ast.Identifier) {
	if name == nil {
		return
	}
	declarationName := name.Value
	if separator := strings.LastIndexByte(declarationName, '.'); separator >= 0 {
		declarationName = declarationName[separator+1:]
	}
	first, _ := utf8.DecodeRuneInString(strings.TrimLeft(declarationName, "_"))
	if unicode.IsUpper(first) {
		return
	}
	a.addErrorAtTokenWithMetadata(name.Token, diagnostics.InvalidNominalTypeName,
		"use an uppercase initial after any visibility prefix and update references to this type",
		"nominal type %s must begin with an uppercase letter after any visibility prefix", name.Value)
}
