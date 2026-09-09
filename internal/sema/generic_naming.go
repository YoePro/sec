package sema

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateGenericTypeParameterNames enforces uppercase type names after the
// visibility prefix. Parameters stay available for recovery; appendError
// deduplicates diagnostics when a declaration participates in multiple passes.
// Rules: rules/foundations/names_scopes_visibility.md — §12 Visibility prefixes,
// §13 Type naming, §15 Generic parameters;
// rules/declarations/generics.md — §2 Generic parameter syntax.
func (a *Analyzer) validateGenericTypeParameterNames(parameters []*ast.GenericParameter) {
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		name := parameter.Name.Value
		first, _ := utf8.DecodeRuneInString(strings.TrimLeft(name, "_"))
		if unicode.IsUpper(first) {
			continue
		}
		a.addErrorAtTokenWithMetadata(parameter.Name.Token, diagnostics.InvalidGenericParameterName,
			"use an uppercase initial after any visibility prefix and update references to this type parameter",
			"generic type parameter %s must begin with an uppercase letter after any visibility prefix", name)
	}
}
