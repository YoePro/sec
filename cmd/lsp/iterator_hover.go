package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// iteratorHover presents the compiler's resolution at the for keyword, source
// expression head and value binding. It performs no interface/member lookup.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/compiler/compiler_analysis.md — §18(6); rules/tooling/lsp.md — "Hover".
func iteratorHover(text string, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	for _, fact := range analyzer.IteratorResolutions() {
		if sameSourceToken(token, fact.Source) || sameSourceToken(token, fact.Iterable) || sameSourceToken(token, fact.Binding) {
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: fact.Display()}, Range: tokenRange(text, token)}, true
		}
	}
	return hoverResult{}, false
}
