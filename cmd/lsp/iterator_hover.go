package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// iteratorHover presents the compiler's resolution at the for keyword, source
// expression head and value binding. It performs no interface/member lookup.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/compiler/compiler_analysis.md — §18(6); rules/tooling/lsp.md — "Hover";
// rules/collections/collections.md — §§14.2,14.6.
func iteratorHover(text string, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	if fact, known := analyzer.MapIterationAt(token.File, token.Line, token.Column); known && fact.BodyValid {
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: fact.MapEntries.Display()}, Range: tokenRange(text, token)}, true
	}
	for _, fact := range analyzer.IteratorResolutions() {
		if sameSourceToken(token, fact.Source) || sameSourceToken(token, fact.Iterable) || sameSourceToken(token, fact.Binding) {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: fact.Display()}, Range: tokenRange(text, token)}, true
		}
	}
	return hoverResult{}, false
}
