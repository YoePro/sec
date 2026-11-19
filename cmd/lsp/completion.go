package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// completeSource reads recorded compiler type facts from the shared recoverable
// source snapshot. A cache hit reuses analyzed node identities; completion never
// invokes fallback inference or mutates the published analyzer.
// Rules: rules/tooling/lsp.md — "Completion" and "Recovery nodes".
func completeSource(uri string, text string, offset int, overlays ...sourceOverlay) []completionItem {
	if items, recognized := errorPatternCompletionItems(uri, text, offset, firstSourceOverlay(overlays)); recognized {
		return items
	}
	if items, recognized := stateTestCompletionItems(uri, text, offset, firstSourceOverlay(overlays)); recognized {
		return items
	}
	context := completionContextAt(text, offset)
	if !context.Member {
		if items, ok := subjectCompletionItems(uri, text, offset, context, firstSourceOverlay(overlays)); ok {
			return items
		}
	}
	parseText := completionParseText(text, offset, context)

	l := lexer.New(parseText)
	if uri != "" {
		l = lexer.NewWithFile(parseText, pathFromURI(uri))
	}
	p := parser.New(l)
	parseResult := p.Parse()
	fileAST := parseResult.Program
	activeModule := programModulePath(fileAST)
	var targetExpr ast.Expression
	var analyzer *sema.Analyzer
	analyzed := false
	if fileAST != nil && !parseResult.Fatal {
		var release func()
		fileAST, analyzer, release = cachedLSPSourceAnalysis(uri, parseText, fileAST, firstSourceOverlay(overlays))
		defer release()
		analyzed = true
		if context.Member {
			targetExpr = findSelectorLHS(fileAST, text, context.DotOffset)
		}
		if expected, getter, ok := expectedReturnTypeAt(fileAST, analyzer, parseText, offset); ok {
			context.ExpectedType = &expected
			context.EnclosingGetter = getter
		}
	} else {
		analyzer = newLSPAnalyzerWithOverlay(uri, fileAST, firstSourceOverlay(overlays))
	}

	if context.Member {
		if fileAST == nil || !analyzed {
			return []completionItem{}
		}
		if items, ok := testingNamespaceCompletionItems(fileAST, parseText, context.DotOffset, context.Prefix); ok {
			return items
		}
		// `atomic.` is the core namespace unless `atomic` is a value in scope.
		if items, ok := atomicNamespaceCompletionItems(parseText, context.DotOffset, context.Prefix); ok {
			if targetExpr == nil {
				return items
			}
			if _, isValue := analyzer.ResolvedTypeOf(targetExpr); !isValue {
				return items
			}
		}
		if targetExpr == nil {
			return []completionItem{}
		}
		if items, ok := unitMetadataCompletionItems(analyzer, targetExpr, context.Prefix); ok {
			return items
		}
		// A selector written directly as the returned value completes only
		// members of the expected return type (user decision 2026-10-03).
		var expected *sema.Type
		receiverStart := memberReceiverStart(text, context.DotOffset)
		if context.ExpectedType != nil && isReturnValueContext(text[:receiverStart], receiverStart) {
			expected = context.ExpectedType
		} else if valueType, ok := expectedValueTypeAt(fileAST, analyzer, text, context.DotOffset); ok {
			// Typed let, assignment, call argument, struct field and match
			// arm values complete like a returned value.
			expected = &valueType
		} else if left := comparisonLeftOperandAt(fileAST, parseText, receiverStart); left != nil {
			// The right operand of `==`/`!=` completes only members of the
			// left operand's type (decision 2026-10-03).
			if leftType, ok := analyzer.ResolvedTypeOf(left); ok {
				expected = &leftType
			}
		}
		if identifier, ok := targetExpr.(*ast.Identifier); ok {
			if staticType, exists := analyzer.Types()[identifier.Value]; exists {
				return fenceOrderCompletionItems(text, receiverStart, identifier.Value, memberCompletionItems(staticType, analyzer, context.Prefix, true, activeModule, expected))
			}
		}
		exprType, ok := analyzer.ResolvedTypeOf(targetExpr)
		if !ok {
			return []completionItem{}
		}
		items := memberCompletionItems(exprType, analyzer, context.Prefix, false, activeModule, expected)
		// Returning the property itself from its own getter would recurse.
		if receiver, onIdentifier := targetExpr.(*ast.Identifier); expected != nil && onIdentifier && receiver.Value == "self" && context.EnclosingGetter != "" {
			filtered := items[:0]
			for _, item := range items {
				if item.Label != context.EnclosingGetter {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		if receiver, onIdentifier := targetExpr.(*ast.Identifier); onIdentifier && receiver.Value == "self" {
			items = append(items, registerOperationCompletionItems(exprType, context.Prefix)...)
		}
		return items
	}

	if items, ok := attributeCompletionItems(text, offset); ok {
		return items
	}
	context.SourceFile = pathFromURI(uri)
	return globalCompletionItems(text, analyzer, context)
}
