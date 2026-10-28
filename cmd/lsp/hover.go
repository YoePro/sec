package main

import "sec/internal/sema"

// hoverForSource preserves default hover without optional parameter insight.
// Rules: rules/tooling/lsp.md — "Hover"; rules/analysis/parameter_usage_analysis.md — "LSP presentation".
func hoverForSource(uri string, text string, pos position, overlays ...sourceOverlay) (hoverResult, bool) {
	return hoverForSourceWithParameterInsight(uri, text, pos, parameterInsightSettings{}, overlays...)
}

// hoverForSourceWithParameterInsight appends canonical allocation context and
// optional compiler-owned demand facts.
// Rules: rules/analysis/parameter_usage_analysis.md — "LSP presentation";
// rules/tooling/lsp.md — "Hover"; rules/memory/allocation.md — §29(1),(3).
func hoverForSourceWithParameterInsight(uri string, text string, pos position, insight parameterInsightSettings, overlays ...sourceOverlay) (result hoverResult, found bool) {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return hoverResult{}, false
	}
	offset := lineCharToOffset(text, pos.Line, pos.Character)
	if offset < 0 {
		return hoverResult{}, false
	}

	path := pathFromURI(uri)
	program, analyzer, release := cachedLSPAnalysis(uri, text, program, firstSourceOverlay(overlays))
	defer release()
	defer func() {
		if token, ok := sourceTokenAtPosition(uri, text, pos); ok {
			if suffix := allocationOperationHover(analyzer, token); suffix != "" {
				if !found {
					result = hoverResult{Contents: markupContent{Kind: "markdown", Value: "Allocation"}, Range: tokenRange(text, token)}
					found = true
				}
				result.Contents.Value += suffix
			}
		}
		if insight.Hover != "" && insight.Hover != "off" {
			if token, ok := sourceTokenAtPosition(uri, text, pos); ok {
				suffix := parameterInsightHover(analyzer, token, insight.Hover)
				if suffix != "" {
					if !found {
						result = hoverResult{Contents: markupContent{Kind: "markdown", Value: "Parameter demand"}, Range: tokenRange(text, token)}
						found = true
					}
					result.Contents.Value += suffix
				}
			}
		}
	}()
	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		if read, ok := analyzer.UnitMetadataReadAt(token.File, token.Line, token.Column); ok {
			return unitMetadataHover(tokenRange(text, token), read), true
		}
		if hover, ok := compareExchangeResultHoverAt(text, program, analyzer, token); ok {
			return hover, true
		}
		if typ, resolved := analyzer.AtomicTypeAt(token.File, token.Line, token.Column); resolved {
			if hover, ok := atomicTypeHover(tokenRange(text, token), typ); ok {
				return hover, true
			}
		}
		if hover, ok := iteratorHover(text, analyzer, token); ok {
			return hover, true
		}
		if hover, ok := attributeHover(text, program, analyzer, token); ok {
			return hover, true
		}
		if hover, ok := tryExpressionHover(text, program, analyzer, token); ok {
			return hover, true
		}
		if hover, ok := tryAssignmentHover(text, program, analyzer, token); ok {
			return hover, true
		}
		if hover, ok := assertionHover(text, program, analyzer, token); ok {
			return hover, true
		}
		if hover, ok := contextualOperatorHover(text, program, analyzer, sourceTokens(uri, text), token); ok {
			return hover, true
		}
	}

	name, nameStart, nameEnd, ok := identifierAtOffset(text, offset)
	if !ok {
		return hoverResult{}, false
	}
	nameRange := offsetsRange(text, nameStart, nameEnd)
	if construction, openOffset, _, found := constructionAtOffset(program, path, text, offset); found && offset <= openOffset {
		if resolved, ok := analyzer.ResolvedConstructionOf(construction); ok {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: initializerHoverContents(resolved)}, Range: nameRange}, true
		}
	}
	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		if initializer, ok := initializerForToken(analyzer, token); ok {
			resolved := sema.ResolvedConstruction{
				Initializer: initializer,
				Implicit:    false,
			}
			if initializer.ConstructionType != nil {
				resolved.Target = *initializer.ConstructionType
			}
			resolved.ErrorType = initializer.ConstructionError
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: initializerHoverContents(resolved)}, Range: nameRange}, true
		}
	}

	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		if plan, ok := analyzer.RegisterFieldAccessAt(token.File, token.Line, token.Column); ok && plan.Request != nil {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: registerAccessHover(plan)}, Range: nameRange}, true
		}
		if operation, ok := analyzer.RegisterOperationAt(token.File, token.Line, token.Column); ok {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: registerOperationHover(operation)}, Range: nameRange}, true
		}
	}
	if target, ok := implTargetAtOffset(program, analyzer, text, path, offset); ok {
		if name == "self" {
			return typedHover(nameRange, "self", target), true
		}
		if isSelfMemberSelector(text, nameStart) {
			if contents, ok := selfMemberHoverContents(target, name, analyzer.Functions(), program, path); ok {
				contents += callGraphHoverSuffix(analyzer, uri, text, pos)
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
			}
		}
	}
	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		if member, resolved := analyzer.CompilerKnownSymbolAt(token.File, token.Line, token.Column); resolved {
			return compilerKnownMemberHover(nameRange, member), true
		}
		definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
		if len(definitions) == 1 {
			if contents, found := enumVariantHoverContents(analyzer, program, definitions[0]); found {
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
			}
			if contents, found := memberHoverContentsForDefinition(analyzer, definitions[0]); found {
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
			}
		}
	}

	// A local or parameter is presented from the binding Sema resolved at this
	// exact position. The module-wide Symbols table below is keyed by name
	// only, so a same-named parameter in another file or function of the
	// module would otherwise supply the type.
	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		binding, ok := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column)
		if !ok {
			binding, ok = analyzer.ResolvedBindingDeclaredAt(token.File, token.Line, token.Column)
		}
		if ok && binding.Name == name && binding.Type.Kind != "" {
			hover := typedHover(nameRange, binding.Name, binding.Type)
			hover.Contents.Value += unitDerivationHoverSuffix(analyzer, binding.Type)
			return hover, true
		}
	}
	if functions := analyzer.Functions()[name]; len(functions) > 0 && !internalCompilerOverloads(functions) {
		contents := functionHoverContents(functions, program, path)
		contents += callGraphHoverSuffix(analyzer, uri, text, pos)
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
	}
	if symbol, ok := analyzer.Symbols()[name]; ok {
		hover := typedHover(nameRange, symbol.Name, symbol.Type)
		hover.Contents.Value += unitDerivationHoverSuffix(analyzer, symbol.Type)
		return hover, true
	}
	if typ, ok := analyzer.Types()[name]; ok {
		if contents, associatedEnum := compareExchangeResultHoverContents(typ, program); associatedEnum {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
		}
		if hover, atomic := atomicTypeHover(nameRange, typ); atomic {
			return hover, true
		}
		if contents, opaque := mutexTypeHoverContents(typ, program); opaque {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
		}
		if typ.Kind == sema.EnumType {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: enumTypeHoverContents(typ, program)}, Range: nameRange}, true
		}
		hover := typedHover(nameRange, "type "+name+genericHeaderDisplay(typ.GenericParameters, typ.GenericConstraints), typ)
		hover.Contents.Value += unitDerivationHoverSuffix(analyzer, typ)
		return hover, true
	}

	return hoverResult{}, false
}
