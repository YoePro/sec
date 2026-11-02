package main

import (
	"sec/internal/lexer"
	"sec/internal/lsp/features"
	"sec/internal/sema"
)

// semanticDeclarationKinds assigns position-exact token classes to declarations
// so a declaration name cannot inherit the class of an unrelated same-named
// symbol through the lexical fallback map.
//
// Rules:
//   - rules/tooling/lsp.md — "Semantic tokens"
//   - rules/declarations/static.md — "Static declarations in implementations"
func semanticDeclarationKinds(analyzer *sema.Analyzer) map[string]string {
	kinds := map[string]string{}
	set := func(token lexer.Token, kind string) {
		if token.Line <= 0 || token.Column <= 0 {
			return
		}
		kinds[features.ClassificationKey(token.File, token.Line, token.Column)] = kind
	}
	for _, typ := range analyzer.Types() {
		if typ.Kind == sema.EnumType {
			set(typ.DeclarationToken, "enum")
		}
		for _, field := range typ.Fields {
			set(field.Token, "property")
		}
		for _, field := range typ.RegisterFields {
			set(field.Token, "property")
		}
		for _, property := range typ.Properties {
			kind := "property"
			if property.Static {
				kind += " static"
			}
			set(property.Token, kind)
		}
		for _, event := range typ.Events {
			set(event.Token, "event")
		}
		for _, value := range typ.EnumConsts {
			set(value.Token, "enumMember")
		}
	}
	for _, overloads := range analyzer.Functions() {
		for _, function := range overloads {
			kind := "function"
			if function.ImplTarget != "" {
				kind = "method"
			}
			if function.Static {
				kind += " static"
			}
			set(function.Token, kind)
		}
	}
	for _, symbol := range analyzer.Symbols() {
		kind := "variable"
		if symbol.ImplicitMember {
			kind = "property"
		} else if !symbol.Mutable {
			kind += " readonly"
		}
		if !symbol.ImplicitMember && symbol.Storage == sema.StorageOriginStatic {
			kind += " static"
		}
		set(symbol.Token, kind)
	}
	// Local statics outlive a callable body; its restored symbol table need not
	// contain them. The static binding facts remain authoritative.
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		kind := "variable static"
		if !static.Mutable {
			kind += " readonly"
		}
		set(static.Declaration.Name.Token, kind)
	}
	return kinds
}
