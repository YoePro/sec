package main

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
)

// declaredMemberHover uses exact compiler definition identity for qualified
// methods and associated storage. Equal spellings on another owner or local
// cannot supply a type or receiver category. Fields/properties/events are
// handled by their specialized hover presenters before this fallback.
// Rules: rules/tooling/lsp.md — "Hover", "Navigation and references";
// rules/declarations/static.md — §§6,9.
func declaredMemberHover(analyzer *sema.Analyzer, definition lexer.Token, program *ast.Program) (string, bool) {
	for _, overloads := range analyzer.Functions() {
		for _, function := range overloads {
			if function.ImplTarget != "" && len(function.GenericSubstitution) == 0 && sameSourceToken(function.Token, definition) {
				return functionHoverContents([]sema.Function{function}, program, definition.File), true
			}
		}
	}
	for _, symbol := range analyzer.Symbols() {
		if strings.Contains(symbol.Name, ".") && symbol.Storage == sema.StorageOriginStatic && sameSourceToken(symbol.Token, definition) {
			modifier := "static let"
			if symbol.Mutable {
				modifier += " mut"
			}
			return fmt.Sprintf("```sec\n%s %s: %s\n```", modifier, symbol.Name, lspTypeName(symbol.Type)), true
		}
	}
	return "", false
}

// memberHoverContentsForDefinition renders the exact field/property/event or
// callable/storage declaration selected by compiler binding facts.
// Rules: rules/tooling/lsp.md — "Hover"; rules/declarations/static.md — §§6,9.
func memberHoverContentsForDefinition(analyzer *sema.Analyzer, definition lexer.Token, program *ast.Program) (string, bool) {
	if fact, ok := analyzer.GenericParameterFactAt(definition); ok {
		return genericParameterHover(fact), true
	}
	if metadata, ok := analyzer.ResolvedStructFieldAt(definition); ok {
		return structFieldHover(metadata.Field), true
	}
	types := analyzer.Types()
	for _, typ := range types {
		for _, field := range typ.RegisterFields {
			if sameSourceToken(field.Token, definition) {
				return registerFieldHover(field), true
			}
		}
		for _, property := range typ.Properties {
			if sameSourceToken(property.Token, definition) {
				return propertyHoverContents(property), true
			}
		}
		for _, event := range typ.Events {
			if sameSourceToken(event.Token, definition) {
				return fmt.Sprintf("```sec\nevent %s: %s\n```", event.Name, lspTypeName(event.Type)), true
			}
		}
	}
	return declaredMemberHover(analyzer, definition, program)
}
